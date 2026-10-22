package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/coderianx/Skillora/server/internal/config"
	"github.com/coderianx/Skillora/server/internal/handlers/auth"
	"github.com/coderianx/Skillora/server/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

func main() {
	port := flag.String("port", ":8080", "port for run server")

	logger := flag.Bool("logger", false, "logger")

	flag.Parse()

	err := godotenv.Load()

	if err != nil {
		log.Fatal("config: load .env file error", err)
	}

	err = config.InitEnvValues()

	if err != nil {
		log.Fatal("config: init .env values error", err)
	}

	// connect database
	err = store.ConnectDatabase()

	if err != nil {
		log.Fatal("database: connect database error", err)
	}

	err = store.CreateTables()

	if err != nil {
		log.Fatal("database: create tables err", err)
	}

	// New router with chi (i love chi ^_^)
	router := chi.NewRouter()

	if *logger {
		router.Use(middleware.Logger)
	}

	router.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", auth.RegisterHandler)
		r.Post("/auth/login", auth.LoginHandler)
		r.Post("/auth/refresh", auth.RefreshHandler)
	})

	http.ListenAndServe(*port, router)
}
