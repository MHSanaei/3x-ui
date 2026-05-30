/**
 * IP Limit Integration Utilities
 * Helper functions for IP-based access control on frontend
 */

/**
 * Get client IP from backend response
 * Used after successful authentication
 */
export function getClientIPInfo() {
  return {
    ip: window.location.hostname || 'unknown',
    timestamp: Date.now(),
  }
}

/**
 * Display IP limit error message
 */
export function displayIPLimitError(message) {
  const errorMsg = message || 'IP limit exceeded. Please contact administrator.'
  return {
    error: true,
    message: errorMsg,
    type: 'ip_limit_exceeded',
  }
}

/**
 * Fetch and display all client IPs
 */
export async function getClientIPs(clientEmail) {
  try {
    const response = await fetch(`/api/client/ips/${clientEmail}`)
    const data = await response.json()
    return data.ips || []
  } catch (error) {
    console.error('Failed to fetch client IPs:', error)
    return []
  }
}

/**
 * Remove specific IP from client
 */
export async function removeClientIP(clientEmail, ip) {
  try {
    const response = await fetch(`/api/client/ips/${clientEmail}/${ip}`, {
      method: 'DELETE',
    })
    return response.ok
  } catch (error) {
    console.error('Failed to remove IP:', error)
    return false
  }
}

/**
 * Clear all client IPs
 */
export async function clearAllClientIPs(clientEmail) {
  try {
    const response = await fetch(`/api/client/ips/${clientEmail}`, {
      method: 'DELETE',
    })
    return response.ok
  } catch (error) {
    console.error('Failed to clear all IPs:', error)
    return false
  }
}

/**
 * Format IP for display
 */
export function formatIPAddress(ip) {
  return ip || 'N/A'
}

/**
 * Check if IP is IPv4
 */
export function isIPv4(ip) {
  const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/
  return ipv4Regex.test(ip)
}

/**
 * Check if IP is IPv6
 */
export function isIPv6(ip) {
  const ipv6Regex = /^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4})$/
  return ipv6Regex.test(ip)
}
