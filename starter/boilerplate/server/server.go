package server

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/molon/genx/starter/boilerplate/server/config"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Serve(conf *config.Config) error {
	db, dbCloser, err := newDatabase(&conf.Database)
	if err != nil {
		return err
	}
	defer dbCloser.Close()

	var c *cors.Cors
	if conf.DevMode {
		log.Println("Running in development mode, allowing all origins")
		c = cors.New(cors.Options{
			AllowOriginFunc: func(origin string) bool {
				return true
			},
			AllowedMethods: []string{
				http.MethodHead,
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
			},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		})
	} else {
		log.Println("Running in production mode, restricting origins")
		if len(conf.Server.AllowedOrigins) == 0 {
			return errors.New("server.allowedOrigins is required in production mode")
		}
		c = cors.New(cors.Options{
			AllowedOrigins:   conf.Server.AllowedOrigins,
			AllowCredentials: true,
			AllowedMethods:   []string{http.MethodGet, http.MethodPost},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
		})
	}

	graphqlEndpoint := strings.TrimSpace(conf.Server.GraphQLEndpoint)
	playgroundEndpoint := strings.TrimSpace(conf.Server.PlaygroundEndpoint)
	graphqlEndpoint = "/" + strings.TrimPrefix(graphqlEndpoint, "/")
	if playgroundEndpoint != "" {
		playgroundEndpoint = "/" + strings.TrimPrefix(playgroundEndpoint, "/")
	}
	if graphqlEndpoint == playgroundEndpoint {
		return errors.New("graphqlEndpoint and playgroundEndpoint must not be the same")
	}
	mux := http.NewServeMux()
	mux.Handle(graphqlEndpoint, c.Handler(NewGQLHandler(db)))
	if playgroundEndpoint != "" {
		mux.Handle(playgroundEndpoint, playground.Handler("GraphQL playground", graphqlEndpoint))
	}

	httpListener, err := net.Listen("tcp", conf.Server.HTTPAddress)
	if err != nil {
		return errors.Wrapf(err, "failed to listen on %s", conf.Server.HTTPAddress)
	}
	defer httpListener.Close()

	httpAddress := httpListener.Addr().String()
	httpServer := &http.Server{Addr: httpAddress, Handler: mux}
	defer func() {
		log.Println("Shutting down HTTP server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
		}
		if err := httpServer.Close(); err != nil {
			log.Printf("Error closing HTTP server: %v", err)
		}
	}()

	doneC := make(chan error, 2)

	go func() {
		log.Printf("Serving HTTP on http://%s", httpAddress)
		doneC <- httpServer.Serve(httpListener)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %s, initiating shutdown...", sig)
		doneC <- nil
	}()

	return <-doneC
}

func newDatabase(conf *config.DatabaseConfig) (*gorm.DB, io.Closer, error) {
	if conf.DSN == "" {
		return nil, nil, errors.New("database.dsn is required")
	}

	db, err := gorm.Open(
		postgres.New(postgres.Config{
			DSN: conf.DSN,
		}),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			CreateBatchSize:                          100,
			PrepareStmt:                              true,
		},
	)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open database connection")
	}

	if conf.Debug {
		db = db.Debug()
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get database connection")
	}
	if conf.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(conf.MaxIdleConns)
	}
	if conf.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(conf.MaxOpenConns)
	}
	if conf.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(conf.ConnMaxLifetime)
	}
	if conf.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(conf.ConnMaxIdleTime)
	}
	return db, sqlDB, nil
}
