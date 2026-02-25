# GitHub Webhooks: Technical Technical Implementation Guide

This document provides a consolidated technical reference for implementing GitHub Webhook receivers, focusing on security, reliability, and architectural best practices.

## 1. Core Architecture
GitHub Webhooks use an event-driven "push" model as an alternative to API polling.
- **Endpoint**: Must be a public HTTPS URL (verified SSL certificate recommended).
- **Timeout**: The receiver MUST return a `2xx` status code within **10 seconds**. Any logic exceeding this window must be processed asynchronously.
- **Payload Formats**:
  - `application/json`: Payload is in the raw request body.
  - `application/x-www-form-urlencoded`: Payload is sent as a `payload` form parameter.

## 2. Essential HTTP Headers
Receivers should leverage these headers for routing and validation:

| Header | Description |
| :--- | :--- |
| `X-GitHub-Event` | The type of event (e.g., `push`, `pull_request`, `issues`). |
| `X-GitHub-Delivery` | A unique GUID for the delivery. Use this for **idempotency/deduplication**. |
| `X-Hub-Signature-256` | HMAC-SHA256 signature of the payload (the primary security check). |
| `User-Agent` | Always starts with `GitHub-Hookshot/`. |
| `X-GitHub-Hook-ID` | Unique ID of the webhook configuration in GitHub. |

## 3. Security & Validation (CRITICAL)
Endpoints are public; therefore, signature validation is mandatory.

### HMAC-SHA256 Verification
1. Capture the **raw request body** (do not parse JSON yet).
2. Calculate the HMAC-SHA256 hash of the raw body using your **Webhook Secret** as the key.
3. Compare the calculated hash with the value in `X-Hub-Signature-256` (prefixed with `sha256=`).
4. **Warning**: Use **constant-time comparison** function to prevent timing attacks.

### IP Whitelisting
For high-security environments, restrict inbound traffic to GitHub's official IP ranges.
- **Discovery**: Query `GET https://api.github.com/meta`.
- **Field**: Check the `hooks` array for CIDR ranges.

## 4. Implementation Workflow
A robust receiver should follow this sequence:
1. **Listen**: Receive POST request.
2. **Authorize**: Validate `X-Hub-Signature-256`.
3. **Deduplicate**: Check `X-GitHub-Delivery` against a cache (e.g., Redis) to avoid re-processing.
4. **Acknowledge**: Immediately return `202 Accepted` or `200 OK`.
5. **Execute**: Trigger background worker/task for business logic.

## 5. Reliability & Error Recovery
- **Automatic Retries**: GitHub does **not** automatically retry failed deliveries for standard webhooks (except GitHub Apps).
- **Manual Redelivery**: Use the "Recent Deliveries" tab in GitHub UI to replay events within a 3-day window.
- **API Redelivery**: Programmatically trigger redelivery by POSTing to `/repos/{owner}/{repo}/hooks/{hook_id}/deliveries/{delivery_id}/attempts`.

## 6. Infrastructure & Deployment
- **Docker**: Use isolated containers (e.g., `linuxserver/webhook`) to execute scripts based on webhooks.
- **Proxy**: Deploy behind a reverse proxy (Nginx/HAProxy) to handle TLS termination and rate limiting.
- **Private Systems**: For internal servers without a public IP, use:
  - **Tunnels**: `ngrok` or `zrok`.
  - **Relays**: `Smee.io` or `Webhook.site`.
  - **Overlay Networks**: `OpenZiti` for secure direct connectivity.

## 7. Operational Limits
- **Payload Size**: Max **25 MB**. Large pushes or complex events might be dropped if they exceed this.
- **Granularity**: Subscribe only to required events to minimize server load.
