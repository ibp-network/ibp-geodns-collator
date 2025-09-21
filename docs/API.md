# IBP GeoDNS Collator API Documentation

## Base URL
```
http(s)://[host]:9000/api/
```

## Authentication
Currently no authentication required (internal network use)

## Common Query Parameters
- **Date Parameters**: Format `YYYY-MM-DD`
  - `start` - Start date (defaults to today)
  - `end` - End date (defaults to today)

## Endpoints

---

### 📊 Request Statistics

#### GET `/api/requests/country`
Get DNS request statistics grouped by country.

**Query Parameters:**
- `start` (string): Start date 
- `end` (string): End date
- `country` (string): Comma-separated country codes (e.g., "US,GB,DE")
- `asn` (string): Comma-separated ASNs (e.g., "AS13335,AS15169")
- `network` (string): Comma-separated network names
- `service` (string): Comma-separated service names
- `member` (string): Comma-separated member names
- `domain` (string): Comma-separated domain names

**Response:**
```json
[
  {
    "date": "2024-09-20",
    "country": "US",
    "country_name": "United States",
    "requests": 150000
  },
  {
    "date": "2024-09-20",
    "country": "GB",
    "country_name": "United Kingdom",
    "requests": 75000
  }
]
```

---

#### GET `/api/requests/asn`
Get DNS request statistics grouped by ASN/network.

**Query Parameters:**
Same as `/api/requests/country`

**Response:**
```json
[
  {
    "date": "2024-09-20",
    "asn": "AS13335",
    "network": "Cloudflare",
    "requests": 250000
  },
  {
    "date": "2024-09-20",
    "asn": "AS15169",
    "network": "Google",
    "requests": 180000
  }
]
```

---

#### GET `/api/requests/service`
Get DNS request statistics grouped by service/chain.

**Query Parameters:**
Same as `/api/requests/country`

**Response:**
```json
[
  {
    "date": "2024-09-20",
    "domain": "polkadot.dotters.network",
    "service": "Polkadot",
    "requests": 500000
  },
  {
    "date": "2024-09-20",
    "domain": "kusama.dotters.network",
    "service": "Kusama",
    "requests": 320000
  }
]
```

---

#### GET `/api/requests/member`
Get DNS request statistics grouped by member.

**Query Parameters:**
Same as `/api/requests/country`

**Response:**
```json
[
  {
    "date": "2024-09-20",
    "member": "Alice Networks",
    "requests": 450000
  },
  {
    "date": "2024-09-20",
    "member": "Bob Hosting",
    "requests": 380000
  }
]
```

---

#### GET `/api/requests/summary`
Get aggregated request statistics summary.

**Query Parameters:**
- `start` (string): Start date
- `end` (string): End date

**Response:**
```json
{
  "start_date": "2024-09-20",
  "end_date": "2024-09-20",
  "total_requests": 2500000,
  "unique_countries": 45,
  "unique_asns": 120,
  "unique_members": 25,
  "unique_domains": 18
}
```

---

### 🔴 Downtime Tracking

#### GET `/api/downtime/events`
Get historical downtime events.

**Query Parameters:**
- `start` (string): Start date
- `end` (string): End date
- `member` (string): Filter by member name
- `service` (string): Filter by service name
- `domain` (string): Filter by domain
- `check_type` (string): Filter by type ("site", "domain", "endpoint")
- `status` (string): Filter by status ("ongoing", "resolved")

**Response:**
```json
[
  {
    "id": 12345,
    "member_name": "Alice Networks",
    "check_type": "endpoint",
    "check_name": "wss-check",
    "domain_name": "polkadot.dotters.network",
    "endpoint": "wss://polkadot.dotters.network",
    "start_time": "2024-09-20T14:30:00Z",
    "end_time": "2024-09-20T15:45:00Z",
    "duration": "1h 15m",
    "error": "connection timeout",
    "is_ipv6": false,
    "status": "resolved"
  }
]
```

---

#### GET `/api/downtime/current`
Get currently ongoing downtime events.

**Response:**
```json
[
  {
    "id": 12346,
    "member_name": "Bob Hosting",
    "check_type": "site",
    "check_name": "ping",
    "start_time": "2024-09-20T16:00:00Z",
    "duration": "45m",
    "error": "no response",
    "is_ipv6": false,
    "status": "ongoing"
  }
]
```

---

#### GET `/api/downtime/summary`
Get downtime statistics summary.

**Query Parameters:**
- `start` (string): Start date
- `end` (string): End date

**Response:**
```json
{
  "start_date": "2024-09-01",
  "end_date": "2024-09-20",
  "total_events": 45,
  "ongoing_events": 2,
  "resolved_events": 43,
  "affected_members": 12,
  "total_downtime_hours": 128.5,
  "average_downtime_hours": 2.99
}
```

---

### 💰 Billing & SLA

#### GET `/api/billing/breakdown`
Get detailed billing breakdown with SLA adjustments.

**Query Parameters:**
- `month` (integer): Month (1-12)
- `year` (integer): Year (2020-2100)
- `member` (string): Filter by member name
- `include_downtime` (string): Include downtime events ("true"/"false")

**Response:**
```json
{
  "month": "2024-09",
  "members": [
    {
      "name": "Alice Networks",
      "level": 3,
      "services": [
        {
          "name": "Polkadot",
          "base_cost": 500.00,
          "uptime_percentage": 99.95,
          "billed_cost": 499.75,
          "credits": 0.25,
          "meets_sla": true,
          "downtime_events": []
        }
      ],
      "total_base_cost": 1500.00,
      "total_billed": 1498.50,
      "total_credits": 1.50,
      "meets_sla": true
    }
  ],
  "total_base_cost": 25000.00,
  "total_billed": 24850.00,
  "total_credits": 150.00
}
```

---

#### GET `/api/billing/summary`
Get monthly billing summary.

**Response:**
```json
{
  "last_refresh": "2024-09-20T12:00:00Z",
  "total_members": 25,
  "total_services": 450,
  "unique_services": 18,
  "total_base_cost_monthly": 25000.00,
  "current_month_credits": 150.00,
  "current_month_sla_violations": 3,
  "service_distribution": {
    "Polkadot": 25,
    "Kusama": 25,
    "Westend": 20
  }
}
```

---

#### GET `/api/billing/pdfs`
List available billing PDF reports.

**Query Parameters:**
- `year` (string): Filter by year
- `month` (string): Filter by month
- `member` (string): Filter by member name

**Response:**
```json
{
  "total": 3,
  "data": [
    {
      "year": "2024",
      "month": "09",
      "pdfs": [
        {
          "year": "2024",
          "month": "09",
          "is_overview": true,
          "file_name": "2024_09-Monthly_Overview.pdf",
          "file_size": 125000,
          "modified_time": "2024-10-01T00:05:00Z"
        },
        {
          "year": "2024",
          "month": "09",
          "member_name": "Alice Networks",
          "is_overview": false,
          "file_name": "2024_09-IBP-Service_Alice_Networks.pdf",
          "file_size": 85000,
          "modified_time": "2024-10-01T00:05:30Z"
        }
      ]
    }
  ]
}
```

---

#### GET `/api/billing/pdfs/download`
Download a specific billing PDF.

**Query Parameters:**
- `year` (string, required): Year
- `month` (string, required): Month
- `member` (string): Member name (required unless type=overview)
- `type` (string): "overview" for monthly overview

**Response:**
Binary PDF file with appropriate headers:
- `Content-Type: application/pdf`
- `Content-Disposition: attachment; filename="2024_09-Monthly_Overview.pdf"`

---

### 👥 Members

#### GET `/api/members`
Get member information.

**Query Parameters:**
- `name` (string): Get specific member

**Response (all members):**
```json
[
  {
    "name": "Alice Networks",
    "website": "https://alice.network",
    "logo": "https://alice.network/logo.png",
    "level": 3,
    "joined_date": "2023-01-15",
    "region": "eu-west-1",
    "latitude": 51.5074,
    "longitude": -0.1278,
    "service_ipv4": "192.168.1.100",
    "service_ipv6": "2001:db8::1",
    "services": ["Polkadot", "Kusama", "Westend"],
    "active": true,
    "override": false
  }
]
```

**Response (specific member):**
Single member object as above.

---

#### GET `/api/members/stats`
Get detailed statistics for a member.

**Query Parameters:**
- `name` (string, required): Member name
- `start` (string): Start date
- `end` (string): End date

**Response:**
```json
{
  "member_name": "Alice Networks",
  "start_date": "2024-09-01",
  "end_date": "2024-09-20",
  "total_requests": 1500000,
  "total_downtime_events": 3,
  "total_downtime_hours": 2.5,
  "uptime_percentage": 99.96,
  "top_countries": [
    {
      "country": "US",
      "name": "United States",
      "requests": 500000
    },
    {
      "country": "GB",
      "name": "United Kingdom",
      "requests": 250000
    }
  ],
  "service_breakdown": [
    {
      "service": "Polkadot",
      "domain": "polkadot.dotters.network",
      "requests": 750000
    },
    {
      "service": "Kusama",
      "domain": "kusama.dotters.network",
      "requests": 500000
    }
  ]
}
```

---

### 🔗 Services

#### GET `/api/services`
Get service catalog information.

**Query Parameters:**
- `name` (string): Get specific service
- `hierarchy` (string): Return hierarchical view ("true"/"false")

**Response (flat list):**
```json
{
  "services": [
    {
      "name": "Polkadot",
      "display_name": "Polkadot",
      "service_type": "RPC",
      "network_name": "Polkadot",
      "relay_network": "",
      "network_type": "Relay",
      "website_url": "https://polkadot.network",
      "logo_url": "https://polkadot.network/logo.png",
      "description": "Polkadot relay chain",
      "active": true,
      "level_required": 1,
      "resources": {
        "nodes": 2,
        "cores": 8.0,
        "memory": 32.0,
        "disk": 500.0,
        "bandwidth": 1000.0
      },
      "providers": [
        {
          "name": "dotters",
          "rpc_urls": [
            "wss://polkadot.dotters.network",
            "https://polkadot-rpc.dotters.network"
          ]
        }
      ],
      "member_count": 25,
      "total_monthly_cost": 0.0
    }
  ],
  "total": 18
}
```

**Response (hierarchy=true):**
```json
{
  "relay_chains": [
    {
      "relay": {
        "name": "Polkadot",
        "display_name": "Polkadot",
        "service_type": "RPC",
        "network_type": "Relay"
      },
      "system_chains": [
        {
          "name": "AssetHub-Polkadot",
          "display_name": "Asset Hub",
          "relay_network": "Polkadot",
          "network_type": "System"
        }
      ],
      "community_chains": [
        {
          "name": "Moonbeam",
          "display_name": "Moonbeam",
          "relay_network": "Polkadot",
          "network_type": "Community"
        }
      ]
    }
  ],
  "orphans": []
}
```

---

#### GET `/api/services/summary`
Get service statistics summary.

**Response:**
```json
{
  "total_services": 18,
  "active_services": 17,
  "relay_chains": 3,
  "service_types": {
    "RPC": 15,
    "ETHRPC": 3
  },
  "network_types": {
    "Relay": 3,
    "System": 6,
    "Community": 9
  },
  "total_resources": {
    "nodes": 50,
    "cores": 400.0,
    "memory": 1600.0,
    "disk": 25000.0,
    "bandwidth": 50000.0
  }
}
```

---

### 🏥 Health Check

#### GET `/api/health`
Check API server health status.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-09-20T15:30:00Z",
  "version": "v0.4.8",
  "ssl": "enabled"
}
```

---

## Error Responses

All endpoints return standard HTTP status codes:

**400 Bad Request:**
```json
{
  "error": "Invalid date format"
}
```

**404 Not Found:**
```json
{
  "error": "Member not found"
}
```

**500 Internal Server Error:**
```json
{
  "error": "Database error"
}
```

## Notes

- All timestamps are in UTC
- Dates use format: `YYYY-MM-DD`
- All monetary values are in USD
- SLA threshold is 99.99% uptime
- Request counts are aggregated hourly
- PDF generation occurs at 00:05 UTC daily/monthly