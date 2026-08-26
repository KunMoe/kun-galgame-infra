package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"api/internal/app"
	"api/internal/galgameapp"
	"api/internal/infrastructure/cache"
	"api/internal/infrastructure/database"
	searchInfra "api/internal/infrastructure/search"
	"api/internal/middleware"
	v2handler "api/internal/platform/apiv2/handler"
	"api/internal/platform/apiv2/protocol"
	"api/internal/platform/catalog/editspec"
	catHandler "api/internal/platform/catalog/handler"
	catalogPerm "api/internal/platform/catalog/perm"
	"api/internal/platform/catalog/repository"
	catalogSearch "api/internal/platform/catalog/search"
	"api/internal/platform/catalog/service"
	"api/internal/platform/devapi"
	"api/internal/platform/editing"
	newsHandler "api/internal/platform/news/handler"
	newsService "api/internal/platform/news/service"
	"api/internal/platform/permissions"
	siteRepo "api/internal/platform/site/repository"
	storeHandler "api/internal/platform/store/handler"
	storeService "api/internal/platform/store/service"
	"api/internal/platform/store/shortener"
	"api/pkg/config"
	"api/pkg/health"
	"api/pkg/imageclient"
	"api/pkg/logger"
	"api/pkg/oidctoken"

	"github.com/gofiber/fiber/v3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	health.MaybeProbe(cfg.CatalogService.Port, "/healthz")

	logger.Init(cfg.Server.Env)

	application, err := app.New(cfg, app.Options{Name: "kun-catalog"})
	if err != nil {
		slog.Error("app init", "error", err)
		os.Exit(1)
	}

	catalogDB, err := database.NewPostgresDB(cfg.CatalogDatabase)
	if err != nil {
		slog.Error("catalog db connect", "error", err)
		os.Exit(1)
	}

	galgameDB, err := database.NewPostgresDB(cfg.GalgameDatabase)
	if err != nil {
		slog.Error("galgame db connect", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := galgameDB.Close(); err != nil {
			slog.Error("close galgame db", "error", err)
		}
	}()

	redirects := repository.NewRedirectRepository(catalogDB.DB())
	resolveSvc := service.NewResolveService(redirects)
	mergeSvc := service.NewMergeService(catalogDB.DB(), resolveSvc,
		repository.NewProposalRepository(catalogDB.DB()), repository.NewRevisionRepository(catalogDB.DB()))
	workSvc := service.NewWorkService(catalogDB.DB(), resolveSvc)
	queueSvc := service.NewAdminQueueService(catalogDB.DB(), mergeSvc)

	readSvc := service.NewReadService(catalogDB.DB())
	statsSvc := service.NewStatsService(catalogDB.DB())
	searchClient, err := searchInfra.NewClient(cfg.Meilisearch)
	if err != nil {
		slog.Error("meilisearch client", "error", err)
		os.Exit(1)
	}
	searcher := catalogSearch.NewIndexer(searchClient)

	application.Fiber.Use(middleware.RequestID())
	application.Fiber.Use(middleware.Logger())
	application.Fiber.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	application.Fiber.Use(middleware.CORS(cfg.Server.CORSOrigin))

	clientRepo := siteRepo.NewOAuthClientRepository(application.DB.DB())
	application.Fiber.Use("/api/v1/catalog", catHandler.S2SAuth(clientRepo))

	tokenVerifier := oidctoken.NewVerifierWithJWKS(cfg.JWT.Secret, cfg.OIDC.JWKSURL)
	application.Fiber.Use("/api/v1/admin/catalog",
		middleware.JWTAuth(tokenVerifier), catHandler.AdminGate(clientRepo))

	application.Fiber.Use(catHandler.UserPrefix,
		middleware.JWTAuth(tokenVerifier), catHandler.UserGate(clientRepo))

	var devCache *cache.RedisCache
	if rc, err := cache.NewRedisCache(cfg.Redis); err != nil {
		slog.Warn("devapi: redis unavailable — rate-limit/quota will fail open", "err", err)
	} else {
		devCache = rc
	}
	devStore := devapi.NewRedisStore(devCache)

	application.Fiber.Use(catHandler.PlaytimePrefix,
		middleware.JWTAuth(tokenVerifier),
		catHandler.PlaytimeGate(catHandler.NewPlaytimeLimiter(devStore)))

	s2sAPI := catHandler.Setup(application.Fiber, resolveSvc, workSvc, readSvc, searcher, statsSvc)
	claimSvc := service.NewClaimLifecycleService(catalogDB.DB())
	catHandler.SetupAdmin(application.Fiber, queueSvc, mergeSvc, claimSvc,
		service.NewImageReferenceService(catalogDB.DB()))

	editRegistry := editing.NewRegistry()
	if err := editspec.RegisterAll(editRegistry, catalogDB.DB()); err != nil {
		slog.Error("editing: register catalog entity types", "error", err)
		os.Exit(1)
	}
	editEngine := editing.NewEngine(catalogDB.DB(), editRegistry)
	catHandler.SetupEdit(s2sAPI, editEngine, catHandler.PermResolvers{
		"catalog": catalogPerm.Resolver,
	})
	catHandler.SetupLifecycle(s2sAPI, claimSvc, editEngine, catHandler.PermResolvers{
		"catalog": catalogPerm.Resolver,
	})
	coverVoteSvc := service.NewCoverVoteService(catalogDB.DB())
	catHandler.SetupUser(application.Fiber, coverVoteSvc, editEngine, catHandler.PermResolvers{
		"catalog": catalogPerm.Resolver,
	}, claimSvc, readSvc)

	playtimeSvc := service.NewUserPlaytimeService(catalogDB.DB())
	catHandler.SetupPlaytime(application.Fiber, playtimeSvc)

	catalogSpec, err := json.Marshal(catHandler.SetupCatalogPublicSpec(fiber.New()).OpenAPI())
	if err != nil {
		slog.Error("marshal catalog public spec", "error", err)
		os.Exit(1)
	}
	application.Fiber.Get("/v1/catalog/openapi.json", func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(catalogSpec)
	})

	newsSpec, err := json.Marshal(newsHandler.SetupNewsPublicSpec(fiber.New()).OpenAPI())
	if err != nil {
		slog.Error("marshal news public spec", "error", err)
		os.Exit(1)
	}
	application.Fiber.Get("/v1/news/openapi.json", func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(newsSpec)
	})

	storeSpec, err := json.Marshal(storeHandler.SetupStorePublicSpec(fiber.New()).OpenAPI())
	if err != nil {
		slog.Error("marshal store public spec", "error", err)
		os.Exit(1)
	}
	application.Fiber.Get("/v1/store/openapi.json", func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(storeSpec)
	})

	// kun_news is a SECOND database on a process whose primary job is catalog.
	// An unreachable news database degrades the news face to 503 instead of
	// exiting: the first production deploy necessarily precedes
	// `CREATE DATABASE kun_news`, and a hard failure there would crash-loop the
	// catalog container over a face nobody is calling yet.
	var newsSvc *newsService.PublicService
	var newsAdminSvc *newsService.AdminService
	var newsWriteSvc *newsService.SubmissionService
	if newsDB, err := database.NewPostgresDB(cfg.NewsDatabase); err != nil {
		slog.Warn("news db connect failed — /v1/news degraded to 503", "dbname", cfg.NewsDatabase.DBName, "error", err)
	} else {
		defer func() {
			if err := newsDB.Close(); err != nil {
				slog.Error("close news db", "error", err)
			}
		}()
		newsSvc = newsService.NewPublicService(newsDB.DB(), cfg.ImageService.CDNBase)
		newsAdminSvc = newsService.NewAdminService(newsDB.DB(), cfg.ImageService.CDNBase)
		newsWriteSvc = newsService.NewSubmissionService(newsDB.DB())
	}

	// The moderation face is the human half of the gate 月幕 asked for. It is a
	// separate prefix with its own permission rather than a filter on the public
	// face: the public face's whole contract is that unpublished items are not
	// addressable there.
	newsAdminH := newsHandler.NewAdminHandler(newsAdminSvc)
	application.Fiber.Use(newsHandler.AdminPrefix,
		middleware.JWTAuth(tokenVerifier), newsHandler.AdminGate(clientRepo))
	adminNews := application.Fiber.Group(newsHandler.AdminPrefix)
	adminNews.Get("/stats", newsAdminH.Stats)
	adminNews.Get("/items", newsAdminH.Queue)
	adminNews.Get("/items/:id", newsAdminH.Detail)
	adminNews.Post("/items/:id/decision", newsAdminH.Decide)

	setupPublicCatalog(application, cfg, catalogDB, readSvc, resolveSvc, searcher, statsSvc,
		clientRepo, tokenVerifier, devStore, devCache, newsSvc, newsWriteSvc, editRegistry, playtimeSvc, coverVoteSvc, claimSvc, editEngine)

	galgameapp.MountRetiredPublic(application)

	application.Fiber.Get("/openapi.json", func(c fiber.Ctx) error {
		b, err := json.Marshal(s2sAPI.OpenAPI())
		if err != nil {
			return err
		}
		c.Set("Content-Type", "application/json")
		return c.Send(b)
	})

	permCtx, cancelPerm := context.WithCancel(context.Background())
	defer cancelPerm()
	permissions.NewDistributor(application.DB.DB(), permissions.Live(), nil).Start(permCtx)

	slog.Info("catalog service starting",
		"addr", fmt.Sprintf("%s:%d", cfg.CatalogService.Host, cfg.CatalogService.Port),
		"dbname", cfg.CatalogDatabase.DBName,
	)

	defer func() {
		if err := catalogDB.Close(); err != nil {
			slog.Error("close catalog db", "error", err)
		}
	}()

	if err := application.Run(cfg.CatalogService.Host, cfg.CatalogService.Port); err != nil {
		slog.Error("run", "error", err)
		os.Exit(1)
	}
}

func setupPublicCatalog(
	application *app.App,
	cfg *config.Config,
	catalogDB *database.PostgresDB,
	readSvc *service.ReadService,
	resolveSvc *service.ResolveService,
	searcher *catalogSearch.Indexer,
	statsSvc *service.StatsService,
	clientRepo catHandler.OAuthClientLookup,
	tokenVerifier *oidctoken.Verifier,
	store devapi.Store,
	devCache *cache.RedisCache,
	newsSvc *newsService.PublicService,
	newsWriteSvc *newsService.SubmissionService,
	editRegistry *editing.Registry,
	playtimeSvc *service.UserPlaytimeService,
	coverVoteSvc *service.CoverVoteService,
	claimSvc *service.ClaimLifecycleService,
	editEngine *editing.Engine,
) {
	oauthDB := application.DB.DB()

	repo := devapi.NewRepository(oauthDB)
	mw := devapi.NewMiddleware(repo, store)
	usageRec := devapi.NewUsageRecorder(repo, store)

	publicSvc := service.NewPublicService(catalogDB.DB(), readSvc, resolveSvc, cfg.ImageService.CDNBase)
	var imgCli *imageclient.Client
	if cfg.ImageClient.ClientID != "" && cfg.ImageClient.ClientSecret != "" {
		imgCli = imageclient.New(imageclient.Config{
			BaseURL:      cfg.ImageClient.BaseURL,
			CDNBase:      cfg.ImageService.CDNBase,
			ClientID:     cfg.ImageClient.ClientID,
			ClientSecret: cfg.ImageClient.ClientSecret,
		})
		publicSvc.WithImageMeta(func(ctx context.Context, hashes []string) (map[string]service.ImageMeta, error) {
			raw, err := imgCli.MetaBatch(ctx, hashes)
			if err != nil {
				return nil, err
			}
			out := make(map[string]service.ImageMeta, len(raw))
			for h, m := range raw {
				out[h] = service.ImageMeta{
					Width: m.Width, Height: m.Height, Thumbhash: m.Thumbhash, Sexual: m.Sexual,
				}
			}
			return out, nil
		})
	} else {
		slog.Warn("catalog public face: image client not configured — covers will carry no dimensions/thumbhash and the banner slot stays null")
	}
	// Edit-face byte upload (wave 169): the multipart leg the row-set editors
	// (covers/screenshots) obtain their hashes from. Deliberately NOT imgCli:
	// that identity is the wiki-era client the meta reads still ride, while
	// new bytes must land under the catalog SITE's own client — the scope the
	// daily catalog refping keeps out of the image GC. Unset credentials
	// disable the leg (503) rather than silently falling back to the wrong
	// site, which would strand every editor upload outside the refping sweep.
	var editUpload catHandler.EditImageUpload
	if cfg.CatalogImageClient.ClientID != "" && cfg.CatalogImageClient.ClientSecret != "" {
		editUpload = imageclient.New(imageclient.Config{
			BaseURL:      cfg.CatalogImageClient.BaseURL,
			CDNBase:      cfg.ImageService.CDNBase,
			ClientID:     cfg.CatalogImageClient.ClientID,
			ClientSecret: cfg.CatalogImageClient.ClientSecret,
		}).UploadWithSub
	} else {
		slog.Warn("catalog edit face: catalog image client not configured — editor image upload disabled (503)")
	}
	catHandler.SetupUserEditImages(application.Fiber, editUpload)
	publicSvc.WithWorksSearch(searcher)
	publicH := catHandler.NewPublicHandler(publicSvc, resolveSvc, searcher, statsSvc).
		WithModeration(clientRepo)

	v2API := v2handler.SetupWith(application.Fiber, v2handler.Options{
		Store:            protocol.NewRedisStore(devCache),
		LookupCredential: mw.Lookup,
		LookupUser: func(ctx context.Context, raw string) (v2handler.UserIdentity, error) {
			claims, err := tokenVerifier.Parse(ctx, raw)
			if err != nil {
				return v2handler.UserIdentity{}, err
			}
			return v2handler.UserIdentity{UID: int64(claims.ID), ClientID: claims.ClientID, Roles: claims.Roles}, nil
		},
		LookupSite: func(ctx context.Context, clientID string) (string, error) {
			cl, err := clientRepo.FindByClientID(ctx, clientID)
			if err != nil || cl == nil {
				return "", err
			}
			return cl.CatalogSite, nil
		},
		Catalog: &v2handler.Catalog{
			Public:      publicSvc,
			Resolve:     resolveSvc,
			StatsSvc:    statsSvc,
			News:        newsSvc,
			NewsWrite:   newsWriteSvc,
			Searcher:    searcher,
			EditTypes:   editRegistry,
			Playtime:    playtimeSvc,
			CoverVotes:  coverVoteSvc,
			Claims:      claimSvc,
			Engine:      editEngine,
			EditHistory: service.NewEditHistoryService(catalogDB.DB()),
			Uploads:     v2handler.EditImageUpload(editUpload),
		},
	})
	v2spec, err := json.Marshal(v2API.OpenAPI())
	if err != nil {
		slog.Error("marshal catalog v2 spec", "error", err)
		os.Exit(1)
	}
	application.Fiber.Get("/v2/catalog/openapi.json", func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(v2spec)
	})

	recordUsage := func(face string) fiber.Handler {
		return func(c fiber.Ctx) error {
			err := c.Next()
			if cred := devapi.CredentialFrom(c); cred != nil {
				usageRec.Record(cred, face, c.Route().Path, c.Response().StatusCode())
				go usageRec.TouchLastUsed(context.Background(), cred)
			}
			return err
		}
	}

	// Credential-free: the catalogue-size counters the public site and the
	// developer portal's landing page render before anyone holds a key.
	// Fiber matches its stack in registration order and a Group's handlers are
	// a prefix Use, so this line must stay ABOVE the group — moved below it the
	// route still resolves, silently back behind the key gate.
	application.Fiber.Get("/v1/catalog/stats", publicH.Stats)

	v1 := application.Fiber.Group("/v1/catalog",
		mw.ResolveCredential,
		recordUsage("catalog"),
		mw.RateLimit,
		mw.Quota,
		devapi.RequireScope(devapi.ScopeCatalogRead),
		middleware.ETag(),
	)

	v1.Get("/lookup", publicH.Lookup)
	v1.Post("/lookup/batch", publicH.LookupBatch)
	v1.Post("/resolve", publicH.Resolve)
	v1.Get("/redirects", publicH.Redirects)
	v1.Get("/search", publicH.Search)
	v1.Get("/works", middleware.OptionalJWT(tokenVerifier), publicH.WorksList)
	v1.Get("/works/search", publicH.WorksSearch)
	v1.Get("/changes", publicH.Changes)
	v1.Get("/releases", publicH.Releases)
	v1.Get("/calendar", publicH.Calendar)
	v1.Get("/calendar/pending", publicH.CalendarPending)
	v1.Get("/calendar/tba", publicH.CalendarTBA)
	v1.Get("/labels", publicH.LabelsList)
	v1.Get("/tags", publicH.TagsList)
	v1.Get("/engines", publicH.EnginesList)
	v1.Get("/series", publicH.SeriesList)
	v1.Get("/works/:id", publicH.WorkDetail)
	v1.Get("/works/:id/covers", publicH.WorkCovers)
	v1.Get("/works/:id/screenshots", publicH.WorkScreenshots)
	v1.Get("/works/:id/tags", publicH.WorkTags)
	v1.Get("/works/:id/characters", publicH.WorkCharacters)
	v1.Get("/works/:id/credits", publicH.WorkCredits)
	v1.Get("/works/:id/releases", publicH.WorkReleases)
	v1.Get("/works/:id/intros", publicH.WorkIntros)
	v1.Get("/works/:id/ratings", publicH.WorkRatings)
	v1.Get("/works/:id/relations", publicH.WorkRelations)
	v1.Get("/works/:id/series", publicH.WorkSeries)
	v1.Get("/works/:id/links", publicH.WorkLinks)
	v1.Get("/works/:id/engines", publicH.WorkEngines)
	v1.Get("/names/:id", publicH.Name)
	v1.Get("/characters/:id", publicH.Character)
	v1.Get("/labels/:id", publicH.Label)
	v1.Get("/labels/:id/relation-graph", publicH.LabelRelationGraph)
	v1.Get("/tags/:id", publicH.Tag)
	v1.Get("/engines/:id", publicH.EngineDetail)
	v1.Get("/series/:id", publicH.Series)

	newsH := newsHandler.NewPublicHandler(newsSvc)
	v1news := application.Fiber.Group("/v1/news",
		mw.ResolveCredential,
		// "catalog", not "news": this group has metered under the catalog face
		// since it launched, and renaming it now would split one application's
		// history across two face values in developer_api_usage.
		recordUsage("catalog"),
		mw.RateLimit,
		mw.Quota,
		devapi.RequireScope(devapi.ScopeNewsRead),
	)
	v1news.Get("/sources", newsH.Sources)
	v1news.Get("/", newsH.List)
	v1news.Get("/:id", newsH.Detail)

	var storeMinter storeService.Minter
	if cfg.Store.ShortlinkBaseURL != "" && cfg.Store.ShortlinkAPIKey != "" {
		storeMinter = shortener.New(cfg.Store.ShortlinkBaseURL, cfg.Store.ShortlinkAPIKey)
	} else {
		slog.Warn("store face: link shortener not configured — /v1/store answers 503 (set KUN_STORE_SHORTLINK_BASE_URL and KUN_STORE_SHORTLINK_API_KEY)")
	}
	storeH := storeHandler.NewPublicHandler(storeService.New(oauthDB, storeMinter, storeService.Options{
		AffTemplateManiax:  cfg.Store.AffTemplateManiax,
		AffTemplatePro:     cfg.Store.AffTemplatePro,
		LinkQuotaPerClient: cfg.Store.LinkQuotaPerClient,
	}))
	v1store := application.Fiber.Group("/v1/store",
		mw.ResolveCredential,
		recordUsage("store"),
		mw.RateLimit,
		mw.Quota,
		devapi.RequireScope(devapi.ScopeStoreRead),
	)
	v1store.Get("/purchase-links/:product_id", storeH.PurchaseLinks)
	v1store.Get("/me/stats", storeH.MyStats)

	flushDone := make(chan struct{})
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-flushDone:
				return
			case <-t.C:
				if err := usageRec.Flush(context.Background()); err != nil {
					slog.Warn("devapi usage flush failed", "err", err)
				}
			}
		}
	}()
	application.Fiber.Hooks().OnPreShutdown(func() error {
		close(flushDone)
		if err := usageRec.Flush(context.Background()); err != nil {
			slog.Warn("devapi final usage flush failed", "err", err)
		}
		if devCache != nil {
			_ = devCache.Close()
		}
		return nil
	})
}
