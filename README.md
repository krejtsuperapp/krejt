# KREJT — Super App (Kosovë)

Repo-ja e produktit. Struktura sipas *Final Master Prompt* §74:

```
apps/        customer · driver · partner · pro (Flutter) · business · admin (Next.js)
backend/     Go — monolit modular (api · modules · domain · infrastructure · workers)
packages/    design-system · localization · api-client · models · validation · utilities
infra/       terraform (AWS eu-central-1) · docker · monitoring
docs/        arkitektura · api · siguria · vendosja · DR · operacionet
```

Vendimet e fiksuara (02.09.2026): vetëm Kosovë · sq/en/de · pa P2P · cash + kartë · AWS Frankfurt (Aurora, ECS Fargate, ElastiCache, SQS/SNS, S3) · Cloudflare Business · Google Maps · FCM · Infobip · Postmark · Centrifugo · marka Vjollcë · vetëm dark mode.

Mockup-i i dizajnit (163 ekrane, burimi i UI-së): `../UDHA WEBSITE/krejt/`.

## Faza 0 — infrastruktura

```
cd infra/terraform/envs/dev
terraform init
terraform plan
terraform apply
```

Kërkon profilin `krejt-dev` (`aws configure sso`). Prod-i zbatohet vetëm nga CI.

## Rregulla

- Asnjë sekret në repo. `.env.example` mban vetëm emrat.
- Asnjë burim AWS me dorë — vetëm Terraform.
- Paraja gjithmonë numër i plotë në cent. Serveri është autoritar.
