package discovery

import (
	"context"
	"io/ioutil"
	"net/http"

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

func (s *server) Run() {
	r := gin.Default()

	r.Use(location.Default())

	r.Static("/ipxe/files", "/discovery_files")

	r.GET("/ipxe/boot", s.ipxeBoot)

	r.PUT("/ready", s.ready)
	r.POST("/discover", s.discover)

	_ = r.Run(s.Address)
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

	cmdLine := string(cmdLineBytes)

	u := location.Get(c)

	// TODO: add URL to cmdLine
	cmdLine += " discovery_url=" + u.String()

	c.String(http.StatusOK, "#!ipxe\necho Booting into imaging OS\ninitrd files/linuxkit-agent-initrd.img\nchain files/linuxkit-agent-kernel %s", cmdLine)
}

func (s *server) ready(c *gin.Context) {
	c.String(http.StatusOK, "hello ready")
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
