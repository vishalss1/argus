package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/vishalss1/argus/src/internal/handler"
)

func New(deviceHandler *handler.DeviceHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r.Route("/devices", func(r chi.Router) {
		r.Get("/", deviceHandler.ListDevices)
		r.Post("/", deviceHandler.CreateDevice)
		r.Route("/{deviceID}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				deviceHandler.GetDevice(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Put("/", func(w http.ResponseWriter, r *http.Request) {
				deviceHandler.UpdateDevice(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Post("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
				deviceHandler.RecordHeartbeat(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				deviceHandler.DeleteDevice(w, r, chi.URLParam(r, "deviceID"))
			})
		})
	})

	return r
}
