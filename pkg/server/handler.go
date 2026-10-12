package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/trisacrypto/daybreak/pkg/errors"
	"github.com/trisacrypto/daybreak/pkg/web/scene"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.rtnl.ai/gimlet/htmx"
)

// All server handlers must implement this interface.
type Handler func(context.Context, *gin.Context) *errors.HTTP

// Convert a sever method into a Gin handler function.
func Wrap(name string, handler Handler, attrs ...attribute.KeyValue) gin.HandlerFunc {
	// All route names must start with "api." or "ui." for tracing.
	if !strings.HasPrefix(name, "api.") && !strings.HasPrefix(name, "ui.") {
		panic("handler name must start with api. or ui. and include the version")
	}

	return func(c *gin.Context) {
		// Set up the telemetry span.
		// TODO: add user attributes and HTTP handler attrs
		attrs = append(attrs, attribute.Bool("is_htmx_request", htmx.IsHTMXRequest(c)))

		ctx, span := tracer.Start(c.Request.Context(), name, trace.WithAttributes(attrs...))
		defer span.End()

		// Execute the handler -- this is where all the API logic happens.
		err := handler(ctx, c)

		// Error handling
		if err != nil {
			// Add the error to the context.
			// This will cause the logging middleware to record the error if it is enabled.
			c.Error(err)

			// If this is an HTMX request always return a JSON response.
			if htmx.IsHTMXRequest(c) {
				c.JSON(err.Status, err.Reply)
			} else {
				// Perform content negotiation
				var template string
				switch err.Status {
				case http.StatusNotFound:
					template = "errors/status/404.html"
				case http.StatusMethodNotAllowed:
					template = "errors/status/405.html"
				default:
					template = "errors/status/500.html"
				}

				c.Negotiate(err.Status, gin.Negotiate{
					Offered:  []string{binding.MIMEJSON, binding.MIMEHTML},
					HTMLName: template,
					HTMLData: scene.New(c).Error(err),
					JSONData: err.Reply,
				})
			}
		}

		// Telemetry code
		if span.IsRecording() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, name+" failed")
			} else {
				span.SetStatus(codes.Ok, name+" succeeded")
			}
		}
	}
}
