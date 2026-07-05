export interface ApiResponse<T = unknown> {
  data: T;
  message?: string;
}

export interface ApiError {
  code: number;
  message: string;
  detail?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export interface PaginationParams {
  page?: number;
  per_page?: number;
}
