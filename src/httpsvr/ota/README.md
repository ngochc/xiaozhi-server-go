# OTA Module

## Directory Structure
- `ota.go`: Default OTA service implementation
- `interfaces.go`: OTA service interface definitions
- `server.go`: OTA HTTP service handler
- `README.md`: Module documentation

## Usage
1. Import the OTA service package in your main program.
2. Initialize the OTA service.
3. Configure the `UpdateURL` parameter to specify the WebSocket address.

## OTA Endpoints
- `GET /api/ota/`: Returns OTA service health status and WebSocket URL.
- `POST /api/ota/`: Receives device handshake/registration and returns server time, firmware info, and WebSocket URL.

## API Testing (Apifox / Postman / curl)

You can test the OTA interface using tools like curl or Apifox:

### 1. Test GET Endpoint
- Method: GET
- URL: `http://localhost:8080/api/ota/`
- Expected Response:
```json
{
  "status": "ok",
  "ws": "ws://localhost:8080/ws"
}
```

### 2. Test POST Endpoint
- Method: POST
- URL: `http://localhost:8080/api/ota/`
- Content-Type: application/json
- Sample Body:
```json
{
  "device_id": "your_device_id"
}
```
- Expected Response:
```json
{
  "server_time": "2024-01-01T12:00:00Z",
  "firmware": "v1.0.0",
  "ws": "ws://localhost:8080/ws"
}
```

### 3. Firmware Download
- URL: `http://localhost:8080/ota_bin/{*.bin}`

### 4. CORS Support
CORS is enabled globally to facilitate browser and third-party tooling access.