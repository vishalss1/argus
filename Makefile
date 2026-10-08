.PHONY: swagger k8s-secrets

swagger:
	cd services/core-service && go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs/swagger --parseDependency

k8s-secrets:
	cp deployments/k8s/base/secrets.env.example deployments/k8s/base/secrets.env

