import { AUTH_BASE_URL, BUDGET_BASE_URL } from './config';

export type User = {
  id: string;
  household_id: string;
  email: string;
  display_name: string;
  created_at: string;
};

export type Household = {
  id: string;
  name: string;
  created_at: string;
};

export type AuthResponse = {
  token: string;
  user: User;
  household: Household;
};

export type Category = {
  id: string;
  household_id: string;
  name: string;
  kind: 'income' | 'expense';
  created_at: string;
};

export type TransactionScope = 'personal' | 'shared';

export type Transaction = {
  id: string;
  household_id: string;
  user_id: string;
  category_id?: string;
  scope: TransactionScope;
  amount_cents: number;
  currency: string;
  description?: string;
  occurred_at: string;
  created_at: string;
};

class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  let res: Response;
  try {
    res = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });
  } catch {
    throw new ApiError(0, 'Could not reach the server. Check the API URL in src/config.ts.');
  }

  if (!res.ok) {
    let message = `Request failed (${res.status})`;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      // response wasn't JSON, keep the generic message
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

function authHeader(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}

export function login(email: string, password: string): Promise<AuthResponse> {
  return request<AuthResponse>(`${AUTH_BASE_URL}/login`, {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export type SignupPayload = {
  email: string;
  password: string;
  display_name: string;
  household_name?: string;
  household_id?: string;
};

export function signup(payload: SignupPayload): Promise<AuthResponse> {
  return request<AuthResponse>(`${AUTH_BASE_URL}/signup`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function listCategories(token: string): Promise<Category[]> {
  return request<Category[]>(`${BUDGET_BASE_URL}/categories`, {
    headers: authHeader(token),
  });
}

export function createCategory(
  token: string,
  name: string,
  kind: Category['kind']
): Promise<Category> {
  return request<Category>(`${BUDGET_BASE_URL}/categories`, {
    method: 'POST',
    headers: authHeader(token),
    body: JSON.stringify({ name, kind }),
  });
}

export function listTransactions(
  token: string,
  scope: TransactionScope | 'all' = 'all'
): Promise<Transaction[]> {
  return request<Transaction[]>(`${BUDGET_BASE_URL}/transactions?scope=${scope}`, {
    headers: authHeader(token),
  });
}

export type CreateTransactionPayload = {
  category_id?: string;
  scope: TransactionScope;
  amount_cents: number;
  currency?: string;
  description?: string;
};

export function createTransaction(
  token: string,
  payload: CreateTransactionPayload
): Promise<Transaction> {
  return request<Transaction>(`${BUDGET_BASE_URL}/transactions`, {
    method: 'POST',
    headers: authHeader(token),
    body: JSON.stringify(payload),
  });
}

export { ApiError };
