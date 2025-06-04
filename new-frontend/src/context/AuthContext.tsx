"use client";

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { post, get, ApiResponse } from '@/services/api';

interface AuthUser {
  // Define based on what your backend session stores or what /api/me might return
  username: string;
  // Add other user properties if available
}

interface AuthContextType {
  isAuthenticated: boolean;
  user: AuthUser | null;
  isLoading: boolean;
  login: (username: string, password: string, twoFactorCode?: string) => Promise<ApiResponse<unknown>>;
  logout: () => Promise<void>;
  checkAuthState: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true); // Start true for initial auth check
  const router = useRouter();
  const pathname = usePathname();

  const checkAuthState = async () => {
    setIsLoading(true);
    try {
      // A common pattern is to have a '/api/me' or '/server/me' endpoint
      // that returns user info if authenticated, or 401 if not.
      // For now, we'll assume if we can access a protected route like '/server/status' (even if it fails for other reasons)
      // it implies a session might be active. This is not ideal.
      // A dedicated "me" endpoint is better.
      // Let's try to fetch /server/status as a proxy for being logged in.
      // The actual data isn't used here, just the success of the call.
      const response = await post<unknown>('/server/status', {}); // This endpoint requires login
      if (response.success) {
        // Ideally, this response would contain user details.
        // For now, we'll mock a user object if the call succeeds.
        // This needs to be improved with a proper /api/me endpoint.
        // The current /server/status doesn't return user info, just system status.
        // So, if it succeeds, we know a session is active.
        // We can't get the username from this though.
        // This is a placeholder until a proper "me" endpoint is available.
        setUser({ username: "Authenticated User" }); // Placeholder
        setIsAuthenticated(true);
      } else {
        setUser(null);
        setIsAuthenticated(false);
        // Do not redirect here, let protected route logic handle it
      }
    } catch (error) {
      console.error("Error checking auth state:", error);
      setUser(null);
      setIsAuthenticated(false);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    checkAuthState();
  }, []); // Run once on mount

  const login = async (username: string, password: string, twoFactorCode?: string) => {
    setIsLoading(true);
    const payload: Record<string, string> = { username, password };
    if (twoFactorCode) {
      payload.twoFactorCode = twoFactorCode;
    }
    const response = await post('/login', payload);
    if (response.success) {
      // Again, ideally fetch user data from a /me endpoint after login
      setUser({ username }); // Placeholder
      setIsAuthenticated(true);
      router.push('/dashboard');
    } else {
      setUser(null);
      setIsAuthenticated(false);
    }
    setIsLoading(false);
    return response; // Return the full response for the login page to handle messages
  };

  const logout = async () => {
    setIsLoading(true);
    try {
      await get('/logout'); // Call backend logout
    } catch (error) {
      console.error("Logout API call failed:", error);
      // Still proceed with client-side logout
    } finally {
      setUser(null);
      setIsAuthenticated(false);
      localStorage.removeItem('theme'); // Also clear theme preference on logout
      router.push('/auth/login');
      setIsLoading(false);
    }
  };

  // Effect to redirect if not authenticated and trying to access a protected page
  useEffect(() => {
    if (!isLoading && !isAuthenticated && !pathname.startsWith('/auth')) {
      router.push('/auth/login');
    }
  }, [isLoading, isAuthenticated, pathname, router]);


  return (
    <AuthContext.Provider value={{ isAuthenticated, user, isLoading, login, logout, checkAuthState }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
