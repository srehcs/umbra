import createClient, { type ClientOptions } from "openapi-fetch";
import type { paths } from "./openapi";

export type { paths };

export function createApiClient(options?: ClientOptions) {
  return createClient<paths>(options);
}
