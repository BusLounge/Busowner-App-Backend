# ==============================================================================
# Bus Owner Notification API Test Script
# Usage:
#   .\test_notification_api.ps1 -Token "YOUR_JWT_TOKEN" [-BaseUrl "http://localhost:8080/api/v1"]
# ==============================================================================

param (
    [Parameter(Mandatory = $false)]
    [string]$BaseUrl = "http://localhost:8080/api/v1",

    [Parameter(Mandatory = $false)]
    [string]$Token = ""
)

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  🔔 BUS OWNER NOTIFICATION API TEST SUITE" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "Target Base URL: $BaseUrl" -ForegroundColor Yellow

if (-not $Token) {
    Write-Host "`n⚠️  No JWT Token supplied." -ForegroundColor Yellow
    Write-Host "   Testing unauthenticated behavior (expecting 401 Unauthorized)...`n" -ForegroundColor DarkGray
}

$headers = @{}
if ($Token) {
    $headers["Authorization"] = "Bearer $Token"
}

# Helper function
function Test-Endpoint {
    param (
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [hashtable]$Headers,
        [int]$ExpectedStatus
    )

    Write-Host "[$Method] $Name -> $Url" -NoNewline
    try {
        $response = Invoke-WebRequest -Uri $Url -Method $Method -Headers $Headers -UseBasicParsing -ErrorAction Stop
        $statusCode = [int]$response.StatusCode
        if ($statusCode -eq $ExpectedStatus) {
            Write-Host " [PASS: $statusCode]" -ForegroundColor Green
            Write-Host "   Response: $($response.Content)" -ForegroundColor DarkGray
            return $response.Content
        } else {
            Write-Host " [FAIL: Expected $ExpectedStatus, got $statusCode]" -ForegroundColor Red
            return $null
        }
    } catch {
        if ($_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            if ($statusCode -eq $ExpectedStatus) {
                Write-Host " [PASS: $statusCode as expected]" -ForegroundColor Green
            } else {
                Write-Host " [FAIL: Expected $ExpectedStatus, got $statusCode]" -ForegroundColor Red
            }
        } else {
            Write-Host " [ERROR: $($_.Exception.Message)]" -ForegroundColor Red
        }
        return $null
    }
}

# 1. Test Unauthenticated Access
Write-Host "`n--- 1. Security / Authentication Verification ---" -ForegroundColor Magenta
Test-Endpoint -Name "Unauthorized List Check" -Method "GET" -Url "$BaseUrl/notifications" -Headers @{} -ExpectedStatus 401
Test-Endpoint -Name "Unauthorized Unread Count Check" -Method "GET" -Url "$BaseUrl/notifications/unread-count" -Headers @{} -ExpectedStatus 401

# If token provided, run authenticated suite
if ($Token) {
    Write-Host "`n--- 2. Authenticated Endpoints Verification ---" -ForegroundColor Magenta
    
    # 2. Get Notifications List
    $listJson = Test-Endpoint -Name "Get Notifications List" -Method "GET" -Url "$BaseUrl/notifications?page=1&limit=10" -Headers $headers -ExpectedStatus 200
    
    # 3. Get Unread Count
    Test-Endpoint -Name "Get Unread Count" -Method "GET" -Url "$BaseUrl/notifications/unread-count" -Headers $headers -ExpectedStatus 200

    # Parse first notification ID if available
    $notifId = $null
    if ($listJson) {
        try {
            $parsed = $listJson | ConvertFrom-Json
            if ($parsed.notifications -and $parsed.notifications.Count -gt 0) {
                $notifId = $parsed.notifications[0].id
                Write-Host "   Found sample notification ID: $notifId" -ForegroundColor Cyan
            }
        } catch {}
    }

    # 4. Mark Single Notification as Read
    if ($notifId) {
        Test-Endpoint -Name "Mark Notification As Read" -Method "PATCH" -Url "$BaseUrl/notifications/$notifId/read" -Headers $headers -ExpectedStatus 200
    } else {
        Write-Host "   Skipping MarkAsRead single item test (no notifications exist in this account)." -ForegroundColor DarkGray
    }

    # 5. Mark All Notifications as Read
    Test-Endpoint -Name "Mark All Notifications As Read" -Method "PATCH" -Url "$BaseUrl/notifications/read-all" -Headers $headers -ExpectedStatus 200
}

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "  ✅ Notification API Test Suite Completed" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan
