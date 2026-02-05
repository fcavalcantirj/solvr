export interface Param {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export interface Endpoint {
  method: "GET" | "POST" | "PATCH" | "DELETE";
  path: string;
  description: string;
  auth?: "jwt" | "api_key" | "both" | "none";
  params?: Param[];
  response: string;
}

export interface EndpointGroup {
  name: string;
  description: string;
  endpoints: Endpoint[];
}
