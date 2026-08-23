package problem

import (
	"crypto/rand"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
)

type FieldError struct {
	Pointer   string `json:"pointer,omitempty" doc:"JSON Pointer (RFC 6901) into the request body. Exactly one of pointer, parameter, or header is set."`
	Parameter string `json:"parameter,omitempty" doc:"Query or path parameter name. Exactly one of pointer, parameter, or header is set."`
	Header    string `json:"header,omitempty" doc:"Request header name. Exactly one of pointer, parameter, or header is set."`
	Reason    string `json:"reason" doc:"Field-level reason from the closed reason registry."`
	Detail    string `json:"detail" doc:"English, request-specific. Must not be used as a discriminant."`
}

type Problem struct {
	Type      string       `json:"type" doc:"Stable problem type URI of the form https://developer.nextmoe.dev/problems/{domain}/{kebab-code}." format:"uri"`
	Title     string       `json:"title" doc:"Stable English phrase for this type. Does not vary per request."`
	Status    int          `json:"status" doc:"HTTP status. Matches the response status line."`
	Detail    string       `json:"detail,omitempty" doc:"English, request-specific. Must not be used as a discriminant."`
	Instance  string       `json:"instance,omitempty" doc:"Request path and query string that failed."`
	Code      string       `json:"code" doc:"Top-level error code from the closed registry. UPPER_SNAKE."`
	RequestID string       `json:"request_id" doc:"Same value as X-Request-ID. Prefix req_ plus a 26-character ULID."`
	Errors    []FieldError `json:"errors,omitempty" doc:"Field-level failures. Required and non-empty when code is VALIDATION_FAILED."`
}

func (p *Problem) Error() string {
	if p == nil {
		return ""
	}
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

func (p *Problem) GetStatus() int {
	if p == nil {
		return http.StatusInternalServerError
	}
	return p.Status
}

func (p *Problem) ContentType(ct string) string {
	if ct == "application/json" || ct == "" {
		return "application/problem+json"
	}
	if ct == "application/cbor" {
		return "application/problem+cbor"
	}
	return ct
}

func New(code, requestID, instance, detail string) *Problem {
	def, ok := Lookup(code)
	if !ok {
		def, _ = Lookup(CodeInternalError)
	}
	return &Problem{
		Type:      def.TypeURI(),
		Title:     def.Title,
		Status:    def.Status,
		Detail:    detail,
		Instance:  instance,
		Code:      def.Code,
		RequestID: requestID,
	}
}

func OfStatus(status int, requestID, instance, detail string) *Problem {
	return New(StatusToCode(status), requestID, instance, detail)
}

func FromHuma(ctx huma.Context, status int, msg string, errs ...error) *Problem {
	requestID, instance := "", ""
	if ctx != nil {
		u := ctx.URL()
		instance = u.RequestURI()
		if v := ctx.Header("X-Request-ID"); strings.HasPrefix(v, "req_") {
			requestID = v
		}
	}
	if requestID == "" {
		requestID = "req_" + newULID()
	}
	code := StatusToCode(status)
	if status == http.StatusUnauthorized {
		if ctx == nil || ctx.Header("Authorization") == "" {
			code = CodeMissingCredential
		} else {
			code = CodeInvalidCredential
		}
	}
	p := New(code, requestID, instance, msg)
	for _, err := range errs {
		if err == nil {
			continue
		}
		p.Errors = append(p.Errors, fieldFromHuma(err))
	}
	if len(p.Errors) > 0 && p.Code != CodeValidationFailed && status == http.StatusUnprocessableEntity {
		p = New(CodeValidationFailed, requestID, instance, msg)
		for _, err := range errs {
			if err == nil {
				continue
			}
			p.Errors = append(p.Errors, fieldFromHuma(err))
		}
	}
	return p
}

func WriteFiberError(c fiber.Ctx, err error) error {
	var p *Problem
	if errors.As(err, &p) && p != nil {
		if p.RequestID == "" {
			p.RequestID = RequestID(c)
		}
		if p.Instance == "" {
			p.Instance = Instance(c)
		}
	} else {
		status := http.StatusInternalServerError
		msg := "Internal Server Error"
		var fe *fiber.Error
		if errors.As(err, &fe) && fe != nil {
			status = fe.Code
			msg = fe.Message
		} else if err != nil {
			msg = err.Error()
		}
		p = OfStatus(status, RequestID(c), Instance(c), msg)
	}
	c.Set("X-Request-ID", p.RequestID)
	if err := c.Status(p.Status).JSON(p); err != nil {
		return err
	}
	c.Set("Content-Type", "application/problem+json")
	return nil
}

func Instance(c fiber.Ctx) string {
	path := c.Path()
	if q := c.Request().URI().QueryArgs(); q != nil && q.Len() > 0 {
		return path + "?" + string(q.QueryString())
	}
	return path
}

func RequestID(c fiber.Ctx) string {
	if v, ok := c.Locals("request_id").(string); ok && strings.HasPrefix(v, "req_") {
		return v
	}
	if v := c.Get("X-Request-ID"); strings.HasPrefix(v, "req_") {
		c.Locals("request_id", v)
		return v
	}
	id := "req_" + newULID()
	c.Locals("request_id", id)
	c.Set("X-Request-ID", id)
	return id
}

func RequestIDMiddleware(c fiber.Ctx) error {
	_ = RequestID(c)
	return c.Next()
}

var bodyLoc = regexp.MustCompile(`\[(\d+)]`)

func fieldFromHuma(err error) FieldError {
	var d *huma.ErrorDetail
	if errors.As(err, &d) && d != nil {
		fe := FieldError{Detail: d.Message, Reason: ReasonInvalidFormat}
		loc := d.Location
		switch {
		case strings.HasPrefix(loc, "query."):
			fe.Parameter = strings.TrimPrefix(loc, "query.")
		case strings.HasPrefix(loc, "path."):
			fe.Parameter = strings.TrimPrefix(loc, "path.")
		case strings.HasPrefix(loc, "header."):
			fe.Header = strings.TrimPrefix(loc, "header.")
		case strings.HasPrefix(loc, "body."):
			fe.Pointer = bodyPointer(strings.TrimPrefix(loc, "body."))
		default:
			if loc != "" {
				fe.Parameter = loc
			}
		}
		return fe
	}
	return FieldError{Reason: ReasonInvalidFormat, Detail: err.Error()}
}

func bodyPointer(loc string) string {
	loc = bodyLoc.ReplaceAllString(loc, "/$1")
	loc = strings.ReplaceAll(loc, ".", "/")
	if loc == "" {
		return ""
	}
	if !strings.HasPrefix(loc, "/") {
		loc = "/" + loc
	}
	return loc
}

func newULID() string {
	var raw [16]byte
	ms := uint64(time.Now().UnixMilli())
	raw[0] = byte(ms >> 40)
	raw[1] = byte(ms >> 32)
	raw[2] = byte(ms >> 24)
	raw[3] = byte(ms >> 16)
	raw[4] = byte(ms >> 8)
	raw[5] = byte(ms)
	_, _ = rand.Read(raw[6:])
	return encodeULID(raw)
}

func encodeULID(id [16]byte) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	dst := [26]byte{
		alphabet[(id[0]&224)>>5],
		alphabet[id[0]&31],
		alphabet[(id[1]&248)>>3],
		alphabet[((id[1]&7)<<2)|((id[2]&192)>>6)],
		alphabet[(id[2]&62)>>1],
		alphabet[((id[2]&1)<<4)|((id[3]&240)>>4)],
		alphabet[((id[3]&15)<<1)|((id[4]&128)>>7)],
		alphabet[(id[4]&124)>>2],
		alphabet[((id[4]&3)<<3)|((id[5]&224)>>5)],
		alphabet[id[5]&31],
		alphabet[(id[6]&248)>>3],
		alphabet[((id[6]&7)<<2)|((id[7]&192)>>6)],
		alphabet[(id[7]&62)>>1],
		alphabet[((id[7]&1)<<4)|((id[8]&240)>>4)],
		alphabet[((id[8]&15)<<1)|((id[9]&128)>>7)],
		alphabet[(id[9]&124)>>2],
		alphabet[((id[9]&3)<<3)|((id[10]&224)>>5)],
		alphabet[id[10]&31],
		alphabet[(id[11]&248)>>3],
		alphabet[((id[11]&7)<<2)|((id[12]&192)>>6)],
		alphabet[(id[12]&62)>>1],
		alphabet[((id[12]&1)<<4)|((id[13]&240)>>4)],
		alphabet[((id[13]&15)<<1)|((id[14]&128)>>7)],
		alphabet[(id[14]&124)>>2],
		alphabet[((id[14]&3)<<3)|((id[15]&224)>>5)],
		alphabet[id[15]&31],
	}
	return string(dst[:])
}
