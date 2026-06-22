.PHONY: up down certs test logs status clean

COMPOSE := docker compose
CERT_DIR := certs
CN := wiki.local

# Generate a self-signed cert + key for wiki.local, with a SAN so cfl's TLS
# verification (and SSL_CERT_FILE trust) works without --insecure.
certs:
	mkdir -p $(CERT_DIR)
	openssl req -x509 -newkey rsa:2048 -nodes \
		-keyout $(CERT_DIR)/$(CN).key \
		-out $(CERT_DIR)/$(CN).crt \
		-days 365 \
		-subj "/CN=$(CN)" \
		-addext "subjectAltName=DNS:$(CN),DNS:localhost,IP:127.0.0.1"
	@echo "Wrote $(CERT_DIR)/$(CN).crt and $(CERT_DIR)/$(CN).key"

up: certs
	$(COMPOSE) up -d
	@echo "Stack starting. Confluence first boot takes 3-5 min."
	@echo "Watch readiness: make -C test/e2e status"
	@echo "Then open https://localhost:8443 (accept the self-signed cert),"
	@echo "complete the Confluence setup wizard, and create a Personal Access Token."

status:
	$(COMPOSE) ps
	@echo "--- confluence health ---"
	@curl -fsS http://localhost:8090/status || echo "(confluence not ready yet)"

logs:
	$(COMPOSE) logs -f confluence

down:
	$(COMPOSE) down

clean:
	$(COMPOSE) down -v
	rm -f $(CERT_DIR)/$(CN).crt $(CERT_DIR)/$(CN).key

# Run the e2e tests. Requires the stack to be up, a configured PAT, and the
# CFL_E2E_* environment variables (see README.md).
test:
	cd ../.. && go test ./test/e2e/... -tags=e2e -v -count=1
