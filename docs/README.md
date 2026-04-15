# IBP GeoDNS Collator

Metrics aggregation and billing service for the IBP GeoDNS System v2, providing usage analytics, SLA calculations, and automated billing reports.

## Overview

The IBP GeoDNS Collator aggregates distributed metrics from DNS nodes and monitors to provide:
- Centralized usage statistics collection
- SLA compliance tracking with automatic credit calculations
- Monthly billing PDF generation
- RESTful API for metrics and billing data
- Matrix notifications for outages

## Features

- **Hourly Usage Collection**: Aggregates DNS query statistics from all nodes
- **SLA Monitoring**: Tracks uptime per service with 99.99% threshold
- **Automated Billing**: Generates monthly PDFs with SLA-adjusted costs
- **Real-time API**: Query requests, downtime, and billing data
- **Member Reports**: Individual billing statements with service breakdowns
- **Geographic Analytics**: Traffic distribution by country and ASN

## Architecture

### Core Components

**Billing Engine** (`src/billing/`)
- Resource cost calculations based on IaaS pricing
- SLA credit computation for downtime
- PDF generation for monthly reports
- Service-level and member-level cost aggregation

**API Server** (`src/api/`)
- RESTful endpoints for metrics and billing
- SSL/TLS support with auto-reload
- CORS-enabled for web frontends
- Input validation and SQL injection protection

**NATS Integration**
- Subscribes to consensus finalization events
- Collects usage data via request-reply
- Hourly aggregation from DNS nodes

## Configuration

```json
{
  "System": {
    "WorkDir": "/path/to/workdir/",
    "LogLevel": "Info",
    "ConfigUrls": {
      "StaticDNSConfig": "https://...",
      "MembersConfig": "https://...",
      "ServicesConfig": "https://...",
      "IaasPricingConfig": "https://...",
      "ServicesRequestsConfig": "https://..."
    },
    "ConfigReloadTime": 3600,
    "MinimumOfflineTime": 900
  },
  "Nats": {
    "NodeID": "COLLATOR-01",
    "Url": "nats://server1:4222,nats://server2:4222",
    "User": "collator",
    "Pass": "__SET_ME__"
  },
  "Mysql": {
    "Host": "localhost",
    "Port": "3306",
    "User": "collator",
    "Pass": "__SET_ME__",
    "DB": "collator"
  },
  "Matrix": {
    "HomeServerURL": "https://matrix.example.org",
    "Username": "__SET_ME__",
    "Password": "__SET_ME__",
    "RoomID": "!roomid:matrix.example.org"
  },
  "CollatorApi": {
    "ListenAddress": "0.0.0.0",
    "ListenPort": "9000"
  }
}
```

The complete example configuration lives in `docs/ibpcollator-config.json`.

## API Endpoints

### Request Statistics
- `GET /api/requests/country` - Requests by country
- `GET /api/requests/asn` - Requests by ASN/network
- `GET /api/requests/service` - Requests by service
- `GET /api/requests/member` - Requests by member
- `GET /api/requests/summary` - Aggregated summary

### Downtime Tracking
- `GET /api/downtime/events` - Historical downtime events
- `GET /api/downtime/current` - Currently offline services
- `GET /api/downtime/summary` - Downtime statistics

### Billing & SLA
- `GET /api/billing/breakdown` - Detailed cost breakdown
- `GET /api/billing/summary` - Monthly billing summary
- `GET /api/billing/pdfs` - List available PDF reports
- `GET /api/billing/pdfs/download` - Download specific PDF

### Members & Services
- `GET /api/members` - Member information
- `GET /api/members/stats` - Member statistics
- `GET /api/services` - Service catalog
- `GET /api/services/summary` - Service overview

## SLA Calculations

The collator tracks service availability and applies credits when uptime falls below 99.99%:

```
Uptime % = (Total Hours - Downtime Hours) / Total Hours * 100
Billed Amount = Base Cost * (Uptime % / 100)
SLA Credit = Base Cost - Billed Amount
```

Downtime tracking:
- Site-level outages affect all member services
- Domain/endpoint outages affect specific services
- Overlapping periods are merged to avoid double-counting

## PDF Generation Schedule

- **Daily** (00:05 UTC): Service cost summary
- **Monthly** (1st day, 00:05 UTC): Member billing statements
- **Contents**: Base costs, uptime metrics, SLA credits, downtime events

Generated PDFs are stored in: `{WorkDir}/tmp/YYYY-MM/`

## Building & Running

### Prerequisites
- Go 1.24.x or higher
- MySQL 5.7+ with collator database
- NATS cluster access
- Matrix homeserver (optional)

### Build
```bash
go build -o bin/ibp-collator ./src/IBPCollator.go
```

### Run
```bash
./bin/ibp-collator -config ./config/ibpcollator.json
```

### Docker
```bash
docker build -t ibp-geodns-collator:dev .
docker run --rm \
  -p 9000:9000 \
  -v "$(pwd)/config:/app/config" \
  -v "$(pwd)/tmp:/app/tmp" \
  ibp-geodns-collator:dev
```

## Database Schema

The authoritative schema is `docs/mysql/db.sql`.

Key runtime expectations:
- `member_events.check_type` is stored as a string field and the service normalizes legacy textual values to numeric equivalents at startup.
- `requests` and `member_events` both use de-duplication constraints that differ from the older examples that were previously documented here.
- If you are provisioning or migrating the database, use `docs/mysql/db.sql` rather than older embedded snippets.

## SSL/TLS Support

Enable HTTPS by setting environment variables:
```bash
export SSL_CERT=/path/to/cert.pem
export SSL_KEY=/path/to/key.pem
./bin/ibp-collator -config ./config/ibpcollator.json
```

The collator monitors certificate files and reloads them automatically when updated.

## Monitoring

### Health Check
```bash
curl http://localhost:9000/api/health
```

### Metrics to Track
- Total requests processed
- SLA violations per member
- PDF generation success/failure
- API response times
- Database connection pool status

## Dependencies

- `github.com/ibp-network/ibp-geodns-libs` - Shared libraries
- `github.com/phpdave11/gofpdf` - PDF generation
- NATS messaging system
- MySQL database

## License

See LICENSE file in repository root.