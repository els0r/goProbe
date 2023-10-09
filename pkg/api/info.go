package api

import (
	"net/http"
	"os"

	"github.com/els0r/goProbe/pkg/version"
	"github.com/gin-gonic/gin"
)

// ServiceInfo summarizes the running service's name, version, and commit. If running in
// kubernetes, it will also print the name of the pod which returned the API call
type ServiceInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Pod     string `json:"pod,omitempty"`
}

// ServiceInfoHandler returns a handler that returns the service name, version, and commit
func ServiceInfoHandler(serviceName string) gin.HandlerFunc {
	info := &ServiceInfo{
		Name:    serviceName,
		Version: version.Short(),
		Commit:  version.GitSHA,
	}

	// try to ascertain the running pod's name
	for _, env := range []string{"POD_NAME", "POD", "PODNAME"} {
		podName := os.Getenv(env)
		if podName != "" {
			info.Pod = podName
			break
		}
	}

	return func(c *gin.Context) {
		c.JSON(http.StatusOK, info)
	}
}

// HealthHandler returns a handler that returns a 200 OK response if the server is healthy
func HealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, "healthy")
	}
}

// ReadyHandler returns a handler that returns a 200 OK response if the server is ready
func ReadyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, "ready")
	}
}
