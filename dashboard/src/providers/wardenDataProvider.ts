import type {
  BaseRecord,
  CreateParams,
  CreateResponse,
  CustomParams,
  CustomResponse,
  DataProvider,
  DeleteOneParams,
  DeleteOneResponse,
  GetListParams,
  GetListResponse,
  GetOneParams,
  GetOneResponse,
  UpdateParams,
  UpdateResponse,
} from "@refinedev/core";

type APIEnvelope<T> = {
  code: number;
  message: string;
  data: T;
};

const apiBaseURL =
  import.meta.env.VITE_WARDEN_API_URL?.toString().trim() || "http://127.0.0.1:8080";
const apiToken = import.meta.env.VITE_WARDEN_API_TOKEN?.toString().trim() || "";

const asPath = (path: string) => {
  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }
  if (path.startsWith("/")) {
    return `${apiBaseURL}${path}`;
  }
  return `${apiBaseURL}/${path}`;
};

const toQueryString = (query?: Record<string, unknown>) => {
  if (!query) {
    return "";
  }
  const values = Object.entries(query).reduce((params, [key, value]) => {
    if (value === undefined || value === null) {
      return params;
    }
    params.set(key, `${value}`);
    return params;
  }, new URLSearchParams());
  return values.toString();
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("Accept", "application/json");
  if (init?.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (apiToken !== "" && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${apiToken}`);
  }

  const response = await fetch(asPath(path), {
    ...init,
    headers,
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`request failed: ${response.status} ${response.statusText} ${body}`.trim());
  }

  const raw = (await response.json()) as APIEnvelope<T> | T;
  if ("code" in (raw as object) && "data" in (raw as object)) {
    const wrapped = raw as APIEnvelope<T>;
    if (wrapped.code !== 0) {
      throw new Error(wrapped.message || `api error: ${wrapped.code}`);
    }
    return wrapped.data;
  }

  return raw as T;
}

const unsupportedResource = (resource: string) => {
  throw new Error(`resource is not supported yet: ${resource}`);
};

export const wardenDataProvider: DataProvider = {
  getApiUrl: () => apiBaseURL,
  getList: async <TData extends BaseRecord = BaseRecord>(
    params: GetListParams,
  ): Promise<GetListResponse<TData>> => {
    if (params.resource !== "deployments") {
      unsupportedResource(params.resource);
    }
    const data = await request<TData[]>("/tasks");
    return {
      data,
      total: data.length,
    };
  },
  getOne: async <TData extends BaseRecord = BaseRecord>(
    params: GetOneParams,
  ): Promise<GetOneResponse<TData>> => {
    if (params.resource !== "deployments") {
      unsupportedResource(params.resource);
    }
    const data = await request<TData>(`/tasks/${params.id}`);
    return { data };
  },
  create: async <
    TData extends BaseRecord = BaseRecord,
    TVariables = Record<string, unknown>,
  >(
    params: CreateParams<TVariables>,
  ): Promise<CreateResponse<TData>> => {
    if (params.resource !== "deployments") {
      unsupportedResource(params.resource);
    }
    const data = await request<TData>("/tasks/deploy", {
      method: "POST",
      body: JSON.stringify(params.variables),
    });
    return { data };
  },
  update: async <
    TData extends BaseRecord = BaseRecord,
    TVariables = Record<string, unknown>,
  >(
    params: UpdateParams<TVariables>,
  ): Promise<UpdateResponse<TData>> => {
    if (params.resource !== "deployments") {
      unsupportedResource(params.resource);
    }
    const body = params.variables as { action?: string };
    if (body.action === "stop") {
      const data = await request<TData>(`/tasks/${params.id}/stop`, {
        method: "POST",
      });
      return { data };
    }
    throw new Error("deployments update currently supports only action=stop");
  },
  deleteOne: async <TData extends BaseRecord = BaseRecord, TVariables = Record<string, unknown>>(
    params: DeleteOneParams<TVariables>,
  ): Promise<DeleteOneResponse<TData>> => {
    if (params.resource !== "deployments") {
      unsupportedResource(params.resource);
    }
    const data = await request<TData>(`/tasks/${params.id}/stop`, {
      method: "POST",
    });
    return { data };
  },
  custom: async <
    TData extends BaseRecord = BaseRecord,
    TQuery = unknown,
    TPayload = unknown,
  >(
    params: CustomParams<TQuery, TPayload>,
  ): Promise<CustomResponse<TData>> => {
    const queryText = toQueryString(params.query as Record<string, unknown> | undefined);
    const path = queryText === "" ? params.url : `${params.url}?${queryText}`;
    const data = await request<TData>(path, {
      method: params.method.toUpperCase(),
      headers: params.headers as HeadersInit | undefined,
      body: params.payload ? JSON.stringify(params.payload) : undefined,
    });
    return { data };
  },
};
