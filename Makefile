# KREJT — komanda të përditshme
.PHONY: up down logs test lint fmt build tf-plan tf-apply

up:            ## nis Postgres + Redis + api + worker lokalisht
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f api worker

test:          ## teste njësie + integrimi (kërkon Postgres lokal)
	cd backend && TEST_DATABASE_URL=postgres://krejt:krejt_dev_only@localhost:5432/krejt?sslmode=disable go test -race -count=1 ./...

lint:
	cd backend && gofmt -l . && go vet ./...

fmt:
	cd backend && gofmt -w . && cd ../infra/terraform && terraform fmt -recursive

build:
	cd backend && CGO_ENABLED=0 go build ./...

tf-plan:
	cd infra/terraform/envs/dev && terraform plan -input=false

tf-apply:
	cd infra/terraform/envs/dev && terraform apply -input=false
