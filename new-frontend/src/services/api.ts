// Basic API client setup
// In a real app, this would be more robust, possibly using axios
// and handling base URLs, interceptors for auth tokens, etc.

export interface ApiResponse<T = unknown> {
  success: boolean;
  message?: string;
  data?: T;
  obj?: T; // Based on existing backend responses
}

async function handleResponse<T>(response: Response): Promise<ApiResponse<T>> {
  if (!response.ok) {
    // Try to parse error from backend if available
    try {
      const errorData = await response.json();
      return { success: false, message: errorData.message || 'An unknown error occurred', data: errorData };
    } catch (parseError) {
      console.error('Error parsing JSON error response:', parseError);
      return { success: false, message: `HTTP error! status: ${response.status}. Failed to parse error response.` };
    }
  }
  // The backend sometimes returns data in 'obj' and sometimes directly,
  // and sometimes just a message.
  // For login, it seems to return { success: true, message: "...", obj: null }
  // For getTwoFactorEnable, it returns { success: true, obj: boolean }
  const contentType = response.headers.get("content-type");
  if (contentType && contentType.indexOf("application/json") !== -1) {
    const jsonData = await response.json();
    // Adapt to existing backend structure which might use 'obj' for data
    return { success: jsonData.success !== undefined ? jsonData.success : true, message: jsonData.message, data: jsonData.obj !== undefined ? jsonData.obj : jsonData };
  }
  return { success: true, message: 'Operation successful but no JSON response.' };
}


export async function post<T = unknown>(url: string, body: Record<string, unknown>): Promise<ApiResponse<T>> {
  try {
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
      credentials: 'include', // Important for sending cookies
    });
    return handleResponse<T>(response);
  } catch (error) {
    console.error('API POST error:', error);
    return { success: false, message: error instanceof Error ? error.message : 'Network error' };
  }
}

// GET request for logout, could be added here too if needed for consistency
export async function get<T = unknown>(url: string): Promise<ApiResponse<T>> {
  try {
    const response = await fetch(url, {
      method: 'GET',
      credentials: 'include',
    });
    return handleResponse<T>(response);
  } catch (error) {
    console.error('API GET error:', error);
    return { success: false, message: error instanceof Error ? error.message : 'Network error' };
  }
}
