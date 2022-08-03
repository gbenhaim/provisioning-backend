package main

import (
	"syscall"

	// Performs initialization of DAO implementation, must be initialized before any database packages.
	_ "github.com/RHEnVision/provisioning-backend/internal/dao/sqlx"
	"github.com/RHEnVision/provisioning-backend/internal/jobs"

	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/RHEnVision/provisioning-backend/internal/clients/cloudwatchlogs"
	"github.com/RHEnVision/provisioning-backend/internal/config"
	"github.com/RHEnVision/provisioning-backend/internal/db"
	"github.com/RHEnVision/provisioning-backend/internal/logging"
	m "github.com/RHEnVision/provisioning-backend/internal/middleware"
	"github.com/RHEnVision/provisioning-backend/internal/routes"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func statusOk(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

func main() {
	config.Initialize()

	// initialize stdout logging and AWS clients first
	log.Logger = logging.InitializeStdout()
	cloudwatchlogs.Initialize()

	// initialize cloudwatch using the AWS clients
	logger, clsFunc, err := logging.InitializeCloudwatch(log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing cloudwatch")
	}
	defer clsFunc()
	log.Logger = logger
	logging.DumpConfigForDevelopment()

	// initialize the rest
	err = db.Initialize("public")
	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing database")
	}

	// initialize the job queue
	ctx := context.Background()
	err = jobs.Initialize(ctx, &logger)
	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing dejq queue")
	}
	if config.Worker.Queue == "memory" {
		jobs.RegisterJobs(&logger)
	}
	jobs.StartDequeueLoop(ctx, &logger)

	// Routes for the main service
	r := chi.NewRouter()
	r.Use(m.RequestID)
	r.Use(m.RequestNum)
	r.Use(m.MetricsMiddleware)
	r.Use(m.LoggerMiddleware(&log.Logger))
	r.Use(render.SetContentType(render.ContentTypeJSON))
	// Unauthenticated routes
	r.Get("/", statusOk)
	// Main routes
	routes.SetupRoutes(r)

	// Routes for metrics
	mr := chi.NewRouter()
	mr.Get("/", statusOk)
	mr.Handle(config.Prometheus.Path, promhttp.Handler())

	log.Info().Msgf("Starting new instance on port %d with prometheus on %d", config.Application.Port, config.Prometheus.Port)
	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", config.Application.Port),
		Handler: r,
	}

	msrv := http.Server{
		Addr:    fmt.Sprintf(":%d", config.Prometheus.Port),
		Handler: mr,
	}

	waitForSignal := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Fatal().Err(err).Msg("Main service shutdown error")
		}
		if err := msrv.Shutdown(context.Background()); err != nil {
			log.Fatal().Err(err).Msg("Metrics service shutdown error")
		}
		close(waitForSignal)
	}()

	go func() {
		if err := msrv.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Fatal().Err(err).Msg("Metrics service listen error")
			}
		}
	}()

	if err := srv.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("Main service listen error")
		}
	}

	<-waitForSignal

	if config.Worker.Queue == "memory" {
		jobs.Queue.Stop()
	}
	log.Info().Msg("Shutdown finished, exiting")
}
