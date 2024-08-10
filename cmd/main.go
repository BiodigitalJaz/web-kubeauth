package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
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

	// Set up session store (using cookies here for simplicity)
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

		// Find the selected context
		var selectedUser, selectedCluster string
		for _, ctx := range kubeConfig.Contexts {
			if ctx.Name == selectedContext {
				selectedUser = ctx.Context.User
				selectedCluster = ctx.Context.Cluster
				break
			}
		}

		// Extract the relevant user and cluster details
		var clientCert, clientKey, caCert []byte
		var clusterServer string
		for _, user := range kubeConfig.Users {
			if user.Name == selectedUser {
				clientCert, _ = base64.StdEncoding.DecodeString(user.User.ClientCertificateData)
				clientKey, _ = base64.StdEncoding.DecodeString(user.User.ClientKeyData)
				break
			}
		}
		for _, cluster := range kubeConfig.Clusters {
			if cluster.Name == selectedCluster {
				caCert, _ = base64.StdEncoding.DecodeString(cluster.Cluster.CertificateAuthorityData)
				clusterServer = cluster.Cluster.Server
				break
			}
		}

		// Load the client certificate and key
		clientCertPair, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to load client key pair: %v", err)
			return
		}

		// Clear sensitive data from memory
		for i := range clientCert {
			clientCert[i] = 0
		}
		for i := range clientKey {
			clientKey[i] = 0
		}
		clientCert = nil
		clientKey = nil

		// Create a CA certificate pool and add the CA cert to it
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			c.String(http.StatusInternalServerError, "Failed to append CA certificate to pool")
			return
		}

		// Clear CA certificate data from memory
		for i := range caCert {
			caCert[i] = 0
		}
		caCert = nil

		// Create a TLS configuration
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{clientCertPair},
			RootCAs:      caCertPool, // Use the CA cert pool to validate the server's certificate
			ClientCAs:    caCertPool,
			ClientAuth:   tls.RequireAndVerifyClientCert, // This ensures client certificate validation
		}

		// Create a new HTTPS client using the provided certificates
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}

		// Use the selected context's cluster server for testing the connection
		resp, err := client.Get(clusterServer)
		if err != nil || resp.StatusCode != http.StatusOK {
			c.String(http.StatusUnauthorized, "Authentication failed: %v", err)
			return
		}

		// If authentication is successful, set a session variable and redirect to the protected home page
		session := sessions.Default(c)
		session.Set("authenticated", true)
		session.Save()

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

		c.HTML(http.StatusOK, "home.html", gin.H{
			"Message": "Welcome to the protected home page!",
		})
	})

	// Load HTML templates
	router.LoadHTMLGlob("templates/*")

	router.Run(":8080")
}
