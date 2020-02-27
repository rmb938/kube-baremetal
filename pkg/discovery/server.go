package discovery

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type server struct {
	Address string
	Client  client.Client

	logger logr.Logger
}

func NewServer(address string, client client.Client) *server {
	return &server{
		Address: address,
		Client:  client,

		logger: ctrl.Log.WithName("discovery-server"),
	}
}

func (s *server) Run(stop <-chan struct{}) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		param := gin.LogFormatterParams{
			Request: c.Request,
			Keys:    c.Keys,
		}

		// Stop timer
		param.TimeStamp = time.Now()
		param.Latency = param.TimeStamp.Sub(start)

		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()

		param.BodySize = c.Writer.Size()

		if raw != "" {
			path = path + "?" + raw
		}

		param.Path = path

		keyValues := []interface{}{
			"status", c.Writer.Status(),
			"latency", param.TimeStamp.Sub(start),
			"client-ip", param.ClientIP,
			"method", param.Method,
			"path", param.Path,
		}

		if len(param.ErrorMessage) > 0 {
			keyValues = append(keyValues, "error", param.ErrorMessage)
		}

		if param.BodySize != -1 {
			keyValues = append(keyValues, "size", param.BodySize)
		}

		s.logger.Info("discovery request", keyValues...)
	})

	r.Use(location.Default())

	r.Static("/ipxe/files", "/discovery_files")

	r.GET("/ipxe/boot", s.ipxeBoot)

	r.POST("/ready", s.ready)
	r.PUT("/heartbeat", s.heartbeat)
	r.POST("/discover", s.discover)

	srv := &http.Server{
		Addr:    s.Address,
		Handler: r,
	}

	go func() {
		s.logger.Info("Starting discovery http server")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error(err, "Error when listening and serving discovery server")
		}
	}()

	<-stop
	s.logger.Info("Stopping discovery http server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}

func (s *server) ipxeBoot(c *gin.Context) {
	systemUUID := c.DefaultQuery("systemUUID", "")

	if len(systemUUID) == 0 {
		c.String(http.StatusOK, "#!ipxe\necho Chaining again with systemUUID\nchain %s", "boot?systemUUID=${uuid}")
		return
	}

	// TODO: check if instance exists with systemUUID
	//  if it does check if need to image or boot into os
	//    if boot into os send ipxe boot into os
	//    else send ipxe to boot into linuxkit
	//  else send ipxe to boot into linuxkit

	cmdLineBytes, err := ioutil.ReadFile("/discovery_files/linuxkit-agent-cmdline")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	cmdLine := strings.TrimSpace(string(cmdLineBytes))

	u := location.Get(c)
	cmdLine += " discovery_url=" + u.String()

	s.logger.Info("booting into agent", "cmdline", cmdLine)

	c.String(http.StatusOK, "#!ipxe\necho Booting into agent\ninitrd files/linuxkit-agent-initrd.img\nchain files/linuxkit-agent-kernel %s", cmdLine)
}

func (s *server) heartbeat(c *gin.Context) {

}

type readyInput struct {
	SystemUUID types.UID `json:"systemUUID"`
	IP         string    `json:"ip"`
}

func (s *server) ready(c *gin.Context) {
	input := &readyInput{}

	if err := c.ShouldBindJSON(input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	var bmh *baremetalv1alpha1.BareMetalHardware

	hardwareList := &baremetalv1alpha1.BareMetalHardwareList{}
	err := s.Client.List(context.Background(), hardwareList, client.MatchingFields{"spec.systemUUID": string(input.SystemUUID)})
	if err != nil {
		if apiError, ok := err.(apierrors.APIStatus); ok {
			if apiError.Status().Code == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(int(apiError.Status().Code), apiError.Status())
			}
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	switch len(hardwareList.Items) {
	case 0:
		// no hardware found so error 404
		c.Status(http.StatusNotFound)
		return
	case 1:
		// one hardware found so we are done
		bmh = &hardwareList.Items[0]
		break
	default:
		// multiple hardware found so error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hardware exists but multiple times, someone messed up."})
		c.Abort()
		return
	}

	bmi := &baremetalv1alpha1.BareMetalInstance{}

	var existingRef *metav1.OwnerReference
	for _, ref := range bmh.OwnerReferences {
		if ref.APIVersion == bmi.APIVersion && ref.Kind == bmi.Kind {
			existingRef = &ref
			break
		}
	}

	if existingRef == nil {
		// No instance found
		c.Status(http.StatusNotFound)
		c.Abort()
		return
	}

	if err = s.Client.Get(context.Background(), types.NamespacedName{Namespace: bmh.Namespace, Name: existingRef.Name}, bmi); err != nil {
		if apierrors.IsNotFound(err) {
			c.Status(http.StatusNotFound)
			c.Abort()
			return
		}

		if apiError, ok := err.(apierrors.APIStatus); ok {
			if apiError.Status().Code == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(int(apiError.Status().Code), apiError.Status())
			}
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	if bmi.Status.Phase != baremetalv1alpha1.BareMetalInstanceStatusPhaseProvisioning && bmi.Status.Phase != baremetalv1alpha1.BareMetalInstanceStatusPhaseCleaning {
		c.Status(http.StatusNotFound)
		c.Abort()
		return
	}

	bmi.Status.AgentInfo = &baremetalv1alpha1.BareMetalInstanceStatusAgentInfo{
		IP: input.IP,
	}
	err = s.Client.Status().Update(context.Background(), bmi)
	if err != nil {
		if apiError, ok := err.(apierrors.APIStatus); ok {
			if apiError.Status().Code == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(int(apiError.Status().Code), apiError.Status())
			}
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	c.Status(http.StatusNoContent)
	c.Abort()
	return

}

type discoveryInput struct {
	SystemUUID types.UID                                    `json:"systemUUID"`
	Hardware   baremetalv1alpha1.BareMetalDiscoveryHardware `json:"hardware"`
}

func (s *server) discover(c *gin.Context) {
	input := &discoveryInput{}

	if err := c.ShouldBindJSON(input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	// TODO: find the ClusterBareMetalHardware
	//  if exists don't do anything else

	hardwareList := &baremetalv1alpha1.BareMetalHardwareList{}
	err := s.Client.List(context.Background(), hardwareList, client.MatchingFields{"spec.systemUUID": string(input.SystemUUID)})
	if err != nil {
		if apiError, ok := err.(apierrors.APIStatus); ok {
			if apiError.Status().Code == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(int(apiError.Status().Code), apiError.Status())
			}
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	switch len(hardwareList.Items) {
	case 0:
		// no hardware found so continue
		break
	case 1:
		// one hardware found so we are done
		c.Status(http.StatusNoContent)
		return
	default:
		// multiple hardware found so error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hardware already exists but multiple times, someone messed up."})
		c.Abort()
		return
	}

	var bmd *baremetalv1alpha1.BareMetalDiscovery

	discoveryList := &baremetalv1alpha1.BareMetalDiscoveryList{}
	err = s.Client.List(context.Background(), discoveryList, client.MatchingFields{"spec.systemUUID": string(input.SystemUUID)})
	if err != nil {
		if apiError, ok := err.(apierrors.APIStatus); ok {
			if apiError.Status().Code == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(int(apiError.Status().Code), apiError.Status())
			}
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	switch len(discoveryList.Items) {
	case 0:
		// TODO: toggle for "secure" discovery
		//  discovery objects must be created by hand first with nil hardware
		//  then they only can be populated when the object already exists
		break
	case 1:
		s.logger.Info("Received discovery for server that is already discovered", "system-uuid", input.SystemUUID)
		bmd = &discoveryList.Items[0]
		break
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "discovery already exists but multiple times, someone messed up."})
		c.Abort()
		return
	}

	// TODO: allow secure discovery by checking if the hardware is already set
	//   if it is do nothing otherwise set it
	if bmd != nil {
		// an existing discovery was found so we don't need to do anything
		c.Status(http.StatusNoContent)
		return
	}

	bmd = &baremetalv1alpha1.BareMetalDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(input.SystemUUID),
		},
		Spec: baremetalv1alpha1.BareMetalDiscoverySpec{
			SystemUUID: input.SystemUUID,
			Hardware:   &input.Hardware,
		},
	}

	err = s.Client.Create(context.Background(), bmd)
	if err != nil {
		if apiError, ok := err.(apierrors.APIStatus); ok {
			if apiError.Status().Code == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			} else {
				c.JSON(int(apiError.Status().Code), apiError.Status())
			}
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		c.Abort()
		return
	}

	c.Status(http.StatusNoContent)
}
