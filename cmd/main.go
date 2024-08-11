package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeConfig struct {
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientKeyData         string `yaml:"client-key-data"`
		} `yaml:"user"`
	} `yaml:"users"`
	Clusters []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
}

func main() {
	router := gin.Default()

	// Set up session store using cookies
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.Sessions("mysession", store))

	// Determine the correct path for the kubeconfig file across different OS
	var kubeConfigPath string
	if runtime.GOOS == "windows" {
		kubeConfigPath = filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
	} else {
		kubeConfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	// Try to read the kubeconfig file
	kubeConfigBytes, err := os.ReadFile(kubeConfigPath)
	if err != nil {
		log.Printf("Warning: Failed to read kubeconfig file: %v. Proceeding without kubeconfig.", err)
		kubeConfigBytes = nil // Set to nil to handle the absence of kubeconfig
	}

	// Initialize kubeConfig variable
	var kubeConfig KubeConfig

	// If kubeconfig file was found, parse it
	if kubeConfigBytes != nil {
		err = yaml.Unmarshal(kubeConfigBytes, &kubeConfig)
		if err != nil {
			log.Fatalf("Failed to parse kubeconfig: %v", err)
		}
	}

	// Display available contexts for the user to select if kubeconfig is present
	router.GET("/", func(c *gin.Context) {
		if kubeConfigBytes != nil && len(kubeConfig.Contexts) > 0 {
			c.HTML(http.StatusOK, "contexts.html", gin.H{
				"Contexts": kubeConfig.Contexts,
			})
		} else {
			c.String(http.StatusOK, "No kubeconfig found or no contexts available. Application running without kubeconfig.")
		}
	})

	// Handle context selection
	router.POST("/select-context", func(c *gin.Context) {
		selectedContext := c.PostForm("context")

		if kubeConfigBytes == nil || len(kubeConfig.Contexts) == 0 {
			c.String(http.StatusBadRequest, "No kubeconfig file or contexts available to select.")
			return
		}

		// Find the selected context details
		var selectedCluster, selectedUser string
		for _, ctx := range kubeConfig.Contexts {
			if ctx.Name == selectedContext {
				selectedCluster = ctx.Context.Cluster
				selectedUser = ctx.Context.User
				break
			}
		}

		// Store only minimal information in the session
		session := sessions.Default(c)
		session.Set("authenticated", true)
		session.Set("user", selectedUser)
		session.Set("cluster", selectedCluster)

		err := session.Save()
		if err != nil {
			log.Printf("Failed to save session: %v\n", err)
			c.String(http.StatusInternalServerError, "Failed to save session")
			return
		}

		c.Redirect(http.StatusFound, "/home")
	})

	// Protected route
	router.GET("/home", func(c *gin.Context) {
		session := sessions.Default(c)
		auth := session.Get("authenticated")

		if auth != true {
			c.Redirect(http.StatusFound, "/")
			return
		}

		// Retrieve minimal data from session
		selectedUser := session.Get("user").(string)

		// Use the client to create a Kubernetes clientset
		clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfigBytes)
		if err != nil {
			log.Printf("Failed to create Kubernetes client config: %v\n", err)
			c.String(http.StatusInternalServerError, "Failed to create Kubernetes client config")
			return
		}

		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			log.Printf("Failed to create Kubernetes REST config: %v\n", err)
			c.String(http.StatusInternalServerError, "Failed to create Kubernetes REST config")
			return
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			log.Printf("Failed to create Kubernetes clientset: %v\n", err)
			c.String(http.StatusInternalServerError, "Failed to create Kubernetes clientset")
			return
		}

		// Query for the user's ClusterRoleBindings
		crbs, err := clientset.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Failed to list ClusterRoleBindings: %v\n", err)
			c.String(http.StatusInternalServerError, "Failed to list ClusterRoleBindings")
			return
		}

		// Check if user is part of the required ClusterRoleBinding
		requiredRoleBinding := os.Getenv("ACCESS_ROLE") // Replace with the required ClusterRoleBinding name
		userAuthorized := false

		for _, crb := range crbs.Items {
			for _, subject := range crb.Subjects {
				if subject.Kind == "User" && subject.Name == selectedUser && crb.RoleRef.Name == requiredRoleBinding {
					userAuthorized = true
					break
				}
			}
			if userAuthorized {
				break
			}
		}

		if !userAuthorized {
			c.String(http.StatusForbidden, "Access denied: You are not authorized to view this page.")
			return
		}

		// Query for RoleBindings (optional, depending on your use case)
		rbs, err := clientset.RbacV1().RoleBindings("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Failed to list RoleBindings: %v\n", err)
			c.String(http.StatusInternalServerError, "Failed to list RoleBindings")
			return
		}

		// Display the home page
		c.HTML(http.StatusOK, "home.html", gin.H{
			"ClusterRoleBindings": crbs.Items,
			"RoleBindings":        rbs.Items,
		})
	})

	// Load HTML templates
	router.LoadHTMLGlob("templates/*")

	router.Run(":8080")
}
