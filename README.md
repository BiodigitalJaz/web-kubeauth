# Gin Kubernetes mTLS Authentication

## Overview

This project demonstrates how to use Mutual TLS (mTLS) authentication in a Golang web application built with the Gin framework. The application leverages a user's `kubeconfig` file to authenticate with a Kubernetes cluster. Users can select a specific context from their `kubeconfig`, which the application then uses to establish a secure connection to the Kubernetes API.

## Features

- **mTLS Authentication**: The application uses mTLS to ensure secure communication between the client and the Kubernetes API. Both the client and server authenticate each other using certificates, providing a high level of security.
  
- **Kubeconfig Integration**: The application automatically detects the user's `kubeconfig` file from the default location (`~/.kube/config` on Linux/macOS and `%USERPROFILE%\.kube\config` on Windows). It parses the file to extract the available contexts, clusters, and certificates.

- **Context Selection**: If the `kubeconfig` file contains multiple contexts, the user is presented with a list of contexts to choose from. The selected context's cluster and user credentials are then used for the authentication process.

- **Protected Routes**: After successful authentication, the user is redirected to a protected home page. Access to this page is restricted to authenticated users only, ensuring that only users who have successfully completed the mTLS authentication can access it.

## Setup

### Prerequisites

- Go 1.16 or higher
- A valid `kubeconfig` file with at least one configured context

### Installation

1. Clone the repository:

    ```bash
    git clone https://github.com/your-username/gin-kubernetes-mtls.git
    cd gin-kubernetes-mtls
    ```

2. Install dependencies:

    ```bash
    go mod tidy
    ```

3. Run the application:

    ```bash
    go run cmd/main.go
    ```

### Running the Application

1. Once the application is running, open your browser and navigate to `http://localhost:8080`.

2. If your `kubeconfig` file is detected and contains multiple contexts, you will be prompted to select a context. 

3. After selecting a context and successfully authenticating, you will be redirected to the protected home page. If authentication fails, an error message will be displayed.

### Project Structure

- `cmd/main.go`: The main application file that handles routing, authentication, and session management.
- `templates/contexts.html`: The HTML template for the context selection page.
- `templates/home.html`: The HTML template for the protected home page.
- `go.mod`: Go module file that manages dependencies.

## How It Works

1. **Detecting the `kubeconfig` File**: The application attempts to locate the user's `kubeconfig` file in the default location. If found, it parses the file to extract the available contexts, clusters, and certificates.

2. **Context Selection**: The user is presented with a list of contexts extracted from the `kubeconfig` file. The user selects one context, and the application uses the associated cluster's server URL and the user’s client certificate to attempt mTLS authentication.

3. **mTLS Authentication**: The client certificate and key from the selected context are used to authenticate the user against the Kubernetes API server. The server's certificate is validated using the CA certificate provided in the `kubeconfig` file.

4. **Session Management**: Upon successful authentication, a session is created, and the user is redirected to the protected home page. The session ensures that the user remains authenticated across different requests.

5. **Protected Routes**: The home page is a protected route that can only be accessed by authenticated users. If an unauthenticated user tries to access this page, they are redirected back to the context selection page.

## Security Considerations

- **Sensitive Data Handling**: The application handles sensitive data, such as client certificates and keys, securely in memory, and ensures that they are cleared from memory after use.
  
- **mTLS Security**: mTLS provides a secure way of ensuring both the client and server authenticate each other. This prevents unauthorized access and ensures that data is encrypted during transmission.

## Contributing

Contributions are welcome! Please fork this repository and submit a pull request with your improvements.

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.