#!/bin/bash
set -e

VERSION="v2.14.14"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

echo "Installing ArgoCD CLI version ${VERSION}..."
echo "Detected OS: ${OS}"
echo "Detected Architecture: ${ARCH}"

case "${OS}" in
    linux)
        case "${ARCH}" in
            x86_64|amd64)
                BINARY="argocd-linux-amd64"
                ;;
            aarch64|arm64)
                BINARY="argocd-linux-arm64"
                ;;
            *)
                echo "Unsupported architecture: ${ARCH}"
                exit 1
                ;;
        esac
        ;;
    darwin)
        case "${ARCH}" in
            x86_64|amd64)
                BINARY="argocd-darwin-amd64"
                ;;
            arm64)
                BINARY="argocd-darwin-arm64"
                ;;
            *)
                echo "Unsupported architecture: ${ARCH}"
                exit 1
                ;;
        esac
        ;;
    *)
        echo "Unsupported OS: ${OS}"
        echo "Please install ArgoCD CLI manually from: https://github.com/argoproj/argo-cd/releases/tag/${VERSION}"
        exit 1
        ;;
esac

URL="https://github.com/argoproj/argo-cd/releases/download/${VERSION}/${BINARY}"
echo "Downloading from: ${URL}"

curl -sSL -o "${BINARY}" "${URL}"

if [ -w "/usr/local/bin" ]; then
    echo "Installing to /usr/local/bin/argocd..."
    install -m 755 "${BINARY}" /usr/local/bin/argocd
else
    echo "Installing to /usr/local/bin/argocd (requires sudo)..."
    sudo install -m 755 "${BINARY}" /usr/local/bin/argocd
fi

rm "${BINARY}"

if command -v argocd &> /dev/null; then
    echo "ArgoCD CLI installed successfully!"
    argocd version --client
else
    echo "Installation completed, but 'argocd' command not found in PATH."
    echo "Please ensure /usr/local/bin is in your PATH."
fi