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
      "MembersConfig": "https://...",
      "ServicesConfig": "https://...",
      "IaasPricingConfig": "https://..."
    }
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
    "User": "ibpcollator",
    "Pass": "__SET_ME__",
    "DB": "ibpcollator"
  },
  "CollatorApi": {
    "ListenAddress": "0.0.0.0",
    "ListenPort": "9000"
  }
}
```

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
go build -o ibp-geodns-collator ./src/IBPCollator.go
```

### Run
```bash
./ibp-geodns-collator -config=/path/to/config.json
```

### Docker
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o collator ./src/IBPCollator.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/collator /collator
ENTRYPOINT ["/collator"]
```

## Database Schema

### requests table
```sql
CREATE TABLE requests (
    date DATE,
    node_id VARCHAR(100),
    domain_name VARCHAR(255),
    member_name VARCHAR(255),
    country_code CHAR(2),
    network_asn VARCHAR(20),
    network_name VARCHAR(255),
    country_name VARCHAR(255),
    is_ipv6 TINYINT(1),
    hits INT,
    PRIMARY KEY (date, node_id, domain_name, member_name, 
                 network_asn, network_name, country_code, 
                 country_name, is_ipv6)
);
```

### member_events table
```sql
CREATE TABLE member_events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    check_type INT,
    check_name VARCHAR(100),
    endpoint TEXT,
    domain_name VARCHAR(255),
    member_name VARCHAR(255),
    status TINYINT(1),
    is_ipv6 TINYINT(1),
    start_time TIMESTAMP,
    end_time TIMESTAMP NULL,
    error TEXT,
    vote_data JSON,
    additional_data JSON,
    UNIQUE KEY unique_event (check_type, check_name, endpoint(255), 
                            domain_name, member_name, is_ipv6, 
                            status, end_time)
);
```

## SSL/TLS Support

Enable HTTPS by setting environment variables:
```bash
export SSL_CERT=/path/to/cert.pem
export SSL_KEY=/path/to/key.pem
./ibp-geodns-collator
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