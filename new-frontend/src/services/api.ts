// Basic API client setup

export interface ApiResponse<T = unknown> { // Default to unknown for better type safety
  success: boolean;
  message?: string;
  data?: T;
  obj?: T;
}

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || '';

async function handleResponse<T>(response: Response): Promise<ApiResponse<T>> {
  if (!response.ok) {
    try {
      const errorData = await response.json();
      return { success: false, message: errorData.message || 'An unknown error occurred', data: errorData as T };
    } catch (parseError) { // Use parseError
      console.error("Failed to parse error JSON:", parseError);
      return { success: false, message: `HTTP error! status: ${response.status} - ${response.statusText}` };
    }
  }
  const contentType = response.headers.get("content-type");
  if (contentType && contentType.indexOf("application/json") !== -1) {
    const jsonData = await response.json();
    // Adapt to backend structure which might use 'obj' or 'data' or be the data itself
    return {
      success: jsonData.success !== undefined ? jsonData.success : true,
      message: jsonData.message,
      data: jsonData.obj !== undefined ? jsonData.obj as T : (jsonData.data !== undefined ? jsonData.data as T : jsonData as T)
      // obj: jsonData.obj as T // Also pass obj if it exists for direct access if needed
    };
  }
  // For non-JSON success responses (e.g. plain text success message, though uncommon for APIs)
  // Or if response.ok is true but no content type or no json
  // This case should be rare for this application's API.
  // If it happens, it means success but no structured data.
  const textData = await response.text(); // Try to get text to include in message
  return { success: true, message: textData || 'Operation successful (non-JSON response).' };
}

export async function post<T = unknown>(url: string, body: Record<string, unknown>): Promise<ApiResponse<T>> { // Changed body type
  const fullUrl = API_BASE_URL ? `${API_BASE_URL}${url}` : url;
  try {
    const response = await fetch(fullUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', },
      body: JSON.stringify(body),
      credentials: 'include',
    });
    return handleResponse<T>(response);
  } catch (error) {
    console.error('API POST error:', fullUrl, error);
    return { success: false, message: error instanceof Error ? error.message : 'Network error or CORS issue. Check browser console and network tab.' };
  }
}

export async function get<T = unknown>(url: string): Promise<ApiResponse<T>> {
  const fullUrl = API_BASE_URL ? `${API_BASE_URL}${url}` : url;
  try {
    const response = await fetch(fullUrl, {
      method: 'GET',
      credentials: 'include',
    });
    return handleResponse<T>(response);
  } catch (error) {
    console.error('API GET error:', fullUrl, error);
    return { success: false, message: error instanceof Error ? error.message : 'Network error or CORS issue. Check browser console and network tab.' };
  }
}
