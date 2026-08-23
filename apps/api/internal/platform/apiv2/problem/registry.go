package problem

import (
	"net/http"
	"regexp"
	"strings"
)

const TypeURIPrefix = "https://developer.nextmoe.dev/problems/"

type Domain string

const (
	DomainPlatform   Domain = "platform"
	DomainCatalog    Domain = "catalog"
	DomainMe         Domain = "me"
	DomainModeration Domain = "moderation"
	DomainNews       Domain = "news"
)

var DomainOrder = []Domain{
	DomainPlatform, DomainCatalog, DomainMe, DomainModeration, DomainNews,
}

type Def struct {
	Code        string
	Domain      Domain
	Status      int
	Title       string
	Description string
}

type ReasonDef struct {
	Reason      string
	Title       string
	Description string
}

const (
	CodeMalformedBody               = "MALFORMED_BODY"
	CodeInvalidParameter            = "INVALID_PARAMETER"
	CodeUnknownEnumValue            = "UNKNOWN_ENUM_VALUE"
	CodeMutuallyExclusiveParameters = "MUTUALLY_EXCLUSIVE_PARAMETERS"
	CodeLimitTooLarge               = "LIMIT_TOO_LARGE"
	CodeTooManyIDs                  = "TOO_MANY_IDS"
	CodeInvalidCursor               = "INVALID_CURSOR"
	CodeUnknownInclude              = "UNKNOWN_INCLUDE"
	CodeUnknownField                = "UNKNOWN_FIELD"
	CodeUnknownSort                 = "UNKNOWN_SORT"
	CodeUnknownFacet                = "UNKNOWN_FACET"
	CodeMissingCredential           = "MISSING_CREDENTIAL"
	CodeInvalidCredential           = "INVALID_CREDENTIAL"
	CodeScopeRequired               = "SCOPE_REQUIRED"
	CodeFirstPartyOnly              = "FIRST_PARTY_ONLY"
	CodeNotFound                    = "NOT_FOUND"
	CodeMethodNotAllowed            = "METHOD_NOT_ALLOWED"
	CodeIdempotencyKeyReused        = "IDEMPOTENCY_KEY_REUSED"
	CodeGone                        = "GONE"
	CodePreconditionFailed          = "PRECONDITION_FAILED"
	CodeUnsupportedMediaType        = "UNSUPPORTED_MEDIA_TYPE"
	CodeValidationFailed            = "VALIDATION_FAILED"
	CodePreconditionRequired        = "PRECONDITION_REQUIRED"
	CodeRateLimited                 = "RATE_LIMITED"
	CodeQuotaExceeded               = "QUOTA_EXCEEDED"
	CodeInternalError               = "INTERNAL_ERROR"
	CodeServiceUnavailable          = "SERVICE_UNAVAILABLE"
	CodeNSFWCapabilityRequired      = "NSFW_CAPABILITY_REQUIRED"
	CodeEntityMerged                = "ENTITY_MERGED"
	CodeUserIdentityRequired        = "USER_IDENTITY_REQUIRED"
	CodeSiteNotBound                = "SITE_NOT_BOUND"
	CodeReleaseCreationDisabled     = "RELEASE_CREATION_DISABLED"
	CodeAlreadyExists               = "ALREADY_EXISTS"
	CodeInvalidStateTransition      = "INVALID_STATE_TRANSITION"
	CodePermissionRequired          = "PERMISSION_REQUIRED"
	CodeTenantMismatch              = "TENANT_MISMATCH"
	CodeDecisionAlreadyMade         = "DECISION_ALREADY_MADE"
)

const (
	ReasonRequired         = "REQUIRED"
	ReasonInvalidFormat    = "INVALID_FORMAT"
	ReasonOutOfRange       = "OUT_OF_RANGE"
	ReasonTooLong          = "TOO_LONG"
	ReasonTooShort         = "TOO_SHORT"
	ReasonTooManyItems     = "TOO_MANY_ITEMS"
	ReasonDuplicateItem    = "DUPLICATE_ITEM"
	ReasonUnknownValue     = "UNKNOWN_VALUE"
	ReasonNotAllowedValue  = "NOT_ALLOWED_VALUE"
	ReasonUnknownReference = "UNKNOWN_REFERENCE"
	ReasonImmutable        = "IMMUTABLE"
	ReasonInconsistentWith = "INCONSISTENT_WITH"
)

var Codes = []Def{
	{CodeMalformedBody, DomainPlatform, http.StatusBadRequest, "Malformed body", "Request body is not valid JSON, or is not a valid instance of the declared media type."},
	{CodeInvalidParameter, DomainPlatform, http.StatusBadRequest, "Invalid parameter", "A parameter is syntactically wrong: a boolean that is not true/false, an integer that is not an integer, a date that is not YYYY-MM-DD."},
	{CodeUnknownEnumValue, DomainPlatform, http.StatusBadRequest, "Unknown enum value", "A closed vocabulary received an unknown token at parse time."},
	{CodeMutuallyExclusiveParameters, DomainPlatform, http.StatusBadRequest, "Mutually exclusive parameters", "Two parameters that cannot appear together were both sent."},
	{CodeLimitTooLarge, DomainPlatform, http.StatusBadRequest, "Limit too large", "limit is greater than 100. The value is not clamped."},
	{CodeTooManyIDs, DomainPlatform, http.StatusBadRequest, "Too many ids", "ids= or refs= named more than 100 items."},
	{CodeInvalidCursor, DomainPlatform, http.StatusBadRequest, "Invalid cursor", "The cursor cannot be parsed or is no longer valid."},
	{CodeUnknownInclude, DomainPlatform, http.StatusBadRequest, "Unknown include", "include= received a token that is not in that collection's vocabulary."},
	{CodeUnknownField, DomainPlatform, http.StatusBadRequest, "Unknown field", "fields= received a top-level key that does not exist."},
	{CodeUnknownSort, DomainPlatform, http.StatusBadRequest, "Unknown sort", "sort= received a key this collection has not declared."},
	{CodeUnknownFacet, DomainPlatform, http.StatusBadRequest, "Unknown facet", "facets= received a name this collection does not support."},
	{CodeMissingCredential, DomainPlatform, http.StatusUnauthorized, "Missing credential", "The request has no Authorization header."},
	{CodeInvalidCredential, DomainPlatform, http.StatusUnauthorized, "Invalid credential", "A credential was sent but it is invalid, expired, or revoked."},
	{CodeScopeRequired, DomainPlatform, http.StatusForbidden, "Scope required", "The credential is valid but lacks the scope this operation needs."},
	{CodeFirstPartyOnly, DomainPlatform, http.StatusForbidden, "First party only", "This operation is only available to first-party clients."},
	{CodeNotFound, DomainPlatform, http.StatusNotFound, "Not found", "Nothing visible exists at this URL."},
	{CodeMethodNotAllowed, DomainPlatform, http.StatusMethodNotAllowed, "Method not allowed", "The path exists but this method does not."},
	{CodeIdempotencyKeyReused, DomainPlatform, http.StatusConflict, "Idempotency key reused", "The same Idempotency-Key was sent with a different request body."},
	{CodeGone, DomainPlatform, http.StatusGone, "Gone", "This URL existed and has been permanently retired."},
	{CodePreconditionFailed, DomainPlatform, http.StatusPreconditionFailed, "Precondition failed", "If-Match did not match the current representation."},
	{CodeUnsupportedMediaType, DomainPlatform, http.StatusUnsupportedMediaType, "Unsupported media type", "The request body media type is not supported."},
	{CodeValidationFailed, DomainPlatform, http.StatusUnprocessableEntity, "Validation failed", "The request is syntactically valid but semantically not. errors[] is present and non-empty."},
	{CodePreconditionRequired, DomainPlatform, http.StatusPreconditionRequired, "Precondition required", "This operation requires If-Match and none was sent."},
	{CodeRateLimited, DomainPlatform, http.StatusTooManyRequests, "Rate limited", "The short-window rate limit was exceeded. Retry-After is in seconds."},
	{CodeQuotaExceeded, DomainPlatform, http.StatusTooManyRequests, "Quota exceeded", "The daily quota is exhausted. Retry-After is until the next window."},
	{CodeInternalError, DomainPlatform, http.StatusInternalServerError, "Internal error", "A bug on our side, including the output of panic recovery."},
	{CodeServiceUnavailable, DomainPlatform, http.StatusServiceUnavailable, "Service unavailable", "A dependency is unavailable. The request may be retried."},
	{CodeNSFWCapabilityRequired, DomainCatalog, http.StatusForbidden, "NSFW capability required", "nsfw=true was requested and this credential does not have the capability. The request is refused, not degraded."},
	{CodeEntityMerged, DomainCatalog, http.StatusNotFound, "Entity merged", "The entity was merged into another. object and current_id are present, and Link rel=canonical is sent."},
	{CodeUserIdentityRequired, DomainMe, http.StatusForbidden, "User identity required", "An application key was used for an operation that needs a user token."},
	{CodeSiteNotBound, DomainMe, http.StatusForbidden, "Site not bound", "The client behind the token is not bound to a catalog site."},
	{CodeReleaseCreationDisabled, DomainMe, http.StatusForbidden, "Release creation disabled", "The proposal tried to create a release. This is a product constraint, not a defect."},
	{CodeAlreadyExists, DomainMe, http.StatusConflict, "Already exists", "The same subject already has a live record for this target."},
	{CodeInvalidStateTransition, DomainMe, http.StatusConflict, "Invalid state transition", "The current state does not allow this transition. detail names the current state and the legal targets."},
	{CodePermissionRequired, DomainModeration, http.StatusForbidden, "Permission required", "The token lacks the permission this decision needs."},
	{CodeTenantMismatch, DomainModeration, http.StatusForbidden, "Tenant mismatch", "The target does not belong to this moderator's catalog site."},
	{CodeDecisionAlreadyMade, DomainModeration, http.StatusConflict, "Decision already made", "This item has already been decided. detail names who decided and when."},
}

var Reasons = []ReasonDef{
	{ReasonRequired, "Required", "A required field is missing or null."},
	{ReasonInvalidFormat, "Invalid format", "The value does not match the expected format (date, URI, hash, id string)."},
	{ReasonOutOfRange, "Out of range", "A numeric value is out of range, including a date outside the allowed interval."},
	{ReasonTooLong, "Too long", "A string is longer than its maxLength."},
	{ReasonTooShort, "Too short", "A string is shorter than its minLength."},
	{ReasonTooManyItems, "Too many items", "An array exceeds its item limit."},
	{ReasonDuplicateItem, "Duplicate item", "An array that must be unique contains a duplicate."},
	{ReasonUnknownValue, "Unknown value", "The value is not in this field's closed vocabulary."},
	{ReasonNotAllowedValue, "Not allowed value", "The value is in the vocabulary but is not accepted in this context."},
	{ReasonUnknownReference, "Unknown reference", "The value refers to an entity that is not visible. Absence, merge, and visibility filtering are not distinguished."},
	{ReasonImmutable, "Immutable", "This field cannot be changed in the current state."},
	{ReasonInconsistentWith, "Inconsistent with", "The value contradicts another field. detail names that field's pointer."},
}

var (
	codeByName   = indexCodes()
	reasonByName = indexReasons()
	NamePattern  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*[A-Z0-9]$`)
)

func indexCodes() map[string]Def {
	m := make(map[string]Def, len(Codes))
	for _, d := range Codes {
		m[d.Code] = d
	}
	return m
}

func indexReasons() map[string]ReasonDef {
	m := make(map[string]ReasonDef, len(Reasons))
	for _, d := range Reasons {
		m[d.Reason] = d
	}
	return m
}

func Lookup(code string) (Def, bool) {
	d, ok := codeByName[code]
	return d, ok
}

func LookupReason(reason string) (ReasonDef, bool) {
	d, ok := reasonByName[reason]
	return d, ok
}

func (d Def) TypeURI() string {
	return TypeURIPrefix + string(d.Domain) + "/" + Kebab(d.Code)
}

func Kebab(code string) string {
	return strings.ToLower(strings.ReplaceAll(code, "_", "-"))
}

func CodeFromKebab(kebab string) string {
	return strings.ToUpper(strings.ReplaceAll(kebab, "-", "_"))
}

func StatusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeInvalidParameter
	case http.StatusUnauthorized:
		return CodeMissingCredential
	case http.StatusForbidden:
		return CodeScopeRequired
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusMethodNotAllowed:
		return CodeMethodNotAllowed
	case http.StatusConflict:
		return CodeAlreadyExists
	case http.StatusGone:
		return CodeGone
	case http.StatusPreconditionFailed:
		return CodePreconditionFailed
	case http.StatusUnsupportedMediaType:
		return CodeUnsupportedMediaType
	case http.StatusUnprocessableEntity:
		return CodeValidationFailed
	case http.StatusPreconditionRequired:
		return CodePreconditionRequired
	case http.StatusTooManyRequests:
		return CodeRateLimited
	case http.StatusServiceUnavailable:
		return CodeServiceUnavailable
	default:
		return CodeInternalError
	}
}
