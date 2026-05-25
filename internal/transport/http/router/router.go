package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/vishalss1/argus/docs/swagger"
	"github.com/vishalss1/argus/internal/transport/http/handler"
	"github.com/vishalss1/argus/internal/transport/http/middleware"
	transportws "github.com/vishalss1/argus/internal/transport/websocket"
)

func New(deviceHandler *handler.DeviceHandler, telemetryHandler *handler.TelemetryHandler, shadowHandler *handler.ShadowHandler, commandHandler *handler.CommandHandler, otaHandler *handler.OTAHandler, ruleHandler *handler.RuleHandler, websocketHandler *transportws.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Metrics)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/docs", http.RedirectHandler("/docs/index.html", http.StatusMovedPermanently).ServeHTTP)
	r.Get("/docs/*", httpSwagger.WrapHandler)
	r.Get("/ws", websocketHandler.ServeHTTP)

	r.Get("/alerts", ruleHandler.ListAlerts)

	r.Route("/rules", func(r chi.Router) {
		r.Get("/", ruleHandler.ListRules)
		r.Post("/", ruleHandler.CreateRule)
		r.Route("/{ruleID}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				ruleHandler.GetRule(w, r, chi.URLParam(r, "ruleID"))
			})
			r.Put("/", func(w http.ResponseWriter, r *http.Request) {
				ruleHandler.UpdateRule(w, r, chi.URLParam(r, "ruleID"))
			})
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				ruleHandler.DeleteRule(w, r, chi.URLParam(r, "ruleID"))
			})
		})
	})

	r.Route("/ota/firmware", func(r chi.Router) {
		r.Get("/", otaHandler.ListFirmware)
		r.Post("/", otaHandler.UploadFirmware)
		r.Get("/{firmwareID}", func(w http.ResponseWriter, r *http.Request) {
			otaHandler.GetFirmware(w, r, chi.URLParam(r, "firmwareID"))
		})
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
			r.Post("/telemetry", func(w http.ResponseWriter, r *http.Request) {
				telemetryHandler.IngestTelemetry(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Get("/commands", func(w http.ResponseWriter, r *http.Request) {
				commandHandler.ListCommands(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Post("/commands", func(w http.ResponseWriter, r *http.Request) {
				commandHandler.SendCommand(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Get("/commands/{commandID}", func(w http.ResponseWriter, r *http.Request) {
				commandHandler.GetCommand(w, r, chi.URLParam(r, "deviceID"), chi.URLParam(r, "commandID"))
			})
			r.Post("/commands/{commandID}/ack", func(w http.ResponseWriter, r *http.Request) {
				commandHandler.AckCommand(w, r, chi.URLParam(r, "deviceID"), chi.URLParam(r, "commandID"))
			})
			r.Post("/commands/{commandID}/nack", func(w http.ResponseWriter, r *http.Request) {
				commandHandler.NackCommand(w, r, chi.URLParam(r, "deviceID"), chi.URLParam(r, "commandID"))
			})
			r.Get("/ota", func(w http.ResponseWriter, r *http.Request) {
				otaHandler.ListDeployments(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Post("/ota", func(w http.ResponseWriter, r *http.Request) {
				otaHandler.DeployFirmware(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Get("/ota/{deploymentID}/manifest", func(w http.ResponseWriter, r *http.Request) {
				otaHandler.GetManifest(w, r, chi.URLParam(r, "deviceID"), chi.URLParam(r, "deploymentID"))
			})
			r.Post("/ota/{deploymentID}/ack", func(w http.ResponseWriter, r *http.Request) {
				otaHandler.AckDeployment(w, r, chi.URLParam(r, "deviceID"), chi.URLParam(r, "deploymentID"))
			})
			r.Post("/ota/{deploymentID}/nack", func(w http.ResponseWriter, r *http.Request) {
				otaHandler.NackDeployment(w, r, chi.URLParam(r, "deviceID"), chi.URLParam(r, "deploymentID"))
			})
			r.Get("/shadow", func(w http.ResponseWriter, r *http.Request) {
				shadowHandler.GetShadow(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Put("/shadow/desired", func(w http.ResponseWriter, r *http.Request) {
				shadowHandler.UpdateDesiredShadow(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Put("/shadow/reported", func(w http.ResponseWriter, r *http.Request) {
				shadowHandler.UpdateReportedShadow(w, r, chi.URLParam(r, "deviceID"))
			})
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				deviceHandler.DeleteDevice(w, r, chi.URLParam(r, "deviceID"))
			})
		})
	})

	return r
}
