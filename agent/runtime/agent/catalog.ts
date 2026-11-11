/**
 * The product catalog, as the reference agent's candidate shelf.
 *
 * A candidate is a product that shares the cut of what the shopper returned:
 * the same brand, the same category and an equal category-specific fit
 * label. The rule is `agent/catalog`'s (Go), which the demo driver and its
 * tests replay from the same Channel3 recordings. A deployment is a Bun
 * container and cannot import a Go package, so this module binds the agent to
 * the catalog service itself and keeps the rule the same: brand, category and
 * an equal fit-handle set, never the line name alone.
 *
 * The answer is Channel3's own product records, unchanged. No size note, no
 * fulfilment label and no match evidence is added to a record: every field
 * the model sees is a field Channel3 returned. Where the size data ends is
 * therefore the model's to say from the record itself — on apparel `variants`
 * is null and sizes live at the retailer's offer URL — and the tool
 * description says so. Real-retailer fulfilment stays simulated in the demo
 * for the same reason `agent/catalog` says it: Channel3 supplies product data
 * and offer links, not the right to take payment for other retailers.
 *
 * The transport is injected, which is how the offline check replays the
 * recordings in `testdata/channel3` with no network and no key.
 */

/** DEFAULT_CATALOG_URL is Channel3's catalog API root when the deployment names none. */
export const DEFAULT_CATALOG_URL = "https://api.trychannel3.com/v1";

/** CATALOG_TIMEOUT_MS bounds one search so a stalled catalog cannot stall a turn. */
const CATALOG_TIMEOUT_MS = 20_000;

/** The error code on a search the deployment cannot make: no catalog key. */
export const CodeCatalogUnconfigured = "catalog-unconfigured";

/** The error code on a search that never reached the catalog. */
export const CodeCatalogUnreachable = "catalog-unreachable";

/** The error code on a search that reached the catalog and never answered in time. */
export const CodeCatalogTimeout = "catalog-timeout";

/** The error code on an answer that is not Channel3's search shape. */
export const CodeCatalogBadResponse = "catalog-bad-response";

/** The error code on a search that names no query. */
export const CodeEmptyQuery = "catalog-empty-query";

/** The error code on a limit that is not a positive integer. */
export const CodeBadLimit = "catalog-bad-limit";

/** The error code on a cut that is missing brand, category, fit handle or values. */
export const CodeInvalidWant = "catalog-invalid-want";

/**
 * CatalogError is one refused, unreachable or unreadable search. It carries
 * the HTTP status and the catalog's own error code where there is one, so a
 * caller can tell a bad key from a service fault without reading the message.
 */
export class CatalogError extends Error {
  /** status is the HTTP status, or 0 when no response arrived. */
  readonly status: number;
  /** code is the catalog's error code, or this module's own for a local fault. */
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(`search_catalog: ${code}${status ? ` (HTTP ${status})` : ""}: ${message}`);
    this.name = "CatalogError";
    this.status = status;
    this.code = code;
  }
}

/** ProductRecord is one Channel3 product, exactly as the catalog returned it. */
export type ProductRecord = Record<string, unknown>;

/**
 * CatalogTransport performs one catalog search and returns Channel3's
 * products. The deployed path is httpCatalogTransport; the offline check
 * replays the recordings in testdata/channel3 instead.
 */
export interface CatalogTransport {
  search(query: string, limit: number): Promise<ProductRecord[]>;
}

/** CatalogHTTPConfig names where the catalog is and which key calls it. */
export interface CatalogHTTPConfig {
  /** url is the catalog API's base, without a trailing slash; the default when empty. */
  url?: string;
  /** key is the Channel3 api key, sent as `x-api-key`. */
  key?: string;
}

/** errorCode reads a `{error:{code}}` answer, or empty. */
function errorCode(parsed: unknown): string {
  if (typeof parsed !== "object" || parsed === null) {
    return "";
  }
  const error = (parsed as { error?: unknown }).error;
  if (typeof error !== "object" || error === null) {
    return "";
  }
  const code = (error as { code?: unknown }).code;
  return typeof code === "string" ? code : "";
}

/**
 * httpCatalogTransport searches Channel3 over HTTP. It refuses to send
 * without a catalog key rather than fall back to anything local, and an
 * answer without a products array is a reported failure, never an empty
 * shelf.
 */
export function httpCatalogTransport(config: CatalogHTTPConfig): CatalogTransport {
  const base = (config.url?.trim() || DEFAULT_CATALOG_URL).replace(/\/+$/, "");
  const key = config.key?.trim() ?? "";
  return {
    async search(query: string, limit: number): Promise<ProductRecord[]> {
      if (key === "") {
        throw new CatalogError(
          0,
          CodeCatalogUnconfigured,
          "CHANNEL3_API_KEY is empty: the deployment carries no catalog key",
        );
      }
      let response: Response;
      try {
        response = await fetch(`${base}/search`, {
          method: "POST",
          headers: {
            "x-api-key": key,
            "content-type": "application/json",
            accept: "application/json",
          },
          body: JSON.stringify({ query, limit }),
          signal: AbortSignal.timeout(CATALOG_TIMEOUT_MS),
        });
      } catch (cause) {
        if (cause instanceof Error && cause.name === "TimeoutError") {
          throw new CatalogError(0, CodeCatalogTimeout, `no answer within ${CATALOG_TIMEOUT_MS} ms`);
        }
        const message = cause instanceof Error ? cause.message : String(cause);
        throw new CatalogError(0, CodeCatalogUnreachable, message);
      }
      const text = await response.text();
      let parsed: unknown;
      try {
        parsed = text === "" ? {} : JSON.parse(text);
      } catch {
        throw new CatalogError(response.status, CodeCatalogBadResponse, text.slice(0, 200));
      }
      if (!response.ok) {
        const code = errorCode(parsed) || `http-${response.status}`;
        const message =
          typeof parsed === "object" && parsed !== null
            ? JSON.stringify((parsed as { error?: unknown }).error ?? parsed).slice(0, 300)
            : text.slice(0, 200);
        throw new CatalogError(response.status, code, message);
      }
      if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        throw new CatalogError(response.status, CodeCatalogBadResponse, "response is not an object");
      }
      const products = (parsed as { products?: unknown }).products;
      if (
        !Array.isArray(products) ||
        products.some((item) => typeof item !== "object" || item === null || Array.isArray(item))
      ) {
        throw new CatalogError(response.status, CodeCatalogBadResponse, "response carries no products array");
      }
      return products as ProductRecord[];
    },
  };
}

/**
 * SearchInput is `search_catalog`'s request: the query and the cut the
 * candidate must share. exclude_skus skips the products behind the belief,
 * such as the returned ones, so a candidate is a new SKU.
 */
export interface SearchInput {
  query: string;
  limit?: number;
  brand_id: string;
  category: string;
  fit_attribute: string;
  fit_values: string[];
  exclude_skus?: string[];
}

/** SearchResult is Channel3's matched products, with no field added to them. */
export interface SearchResult {
  products: ProductRecord[];
}

/** The catalog the agent's tool calls: one search returning the matching cut. */
export interface Catalog {
  search(input: SearchInput): Promise<SearchResult>;
}

/** DefaultLimit is how many products one search asks the catalog for. */
const DefaultLimit = 10;

/** normalizeFit lowercases and trims fit values so Regular equals regular. */
function normalizeFit(values: string[]): Set<string> {
  const set = new Set<string>();
  for (const value of values) {
    const normalized = value.toLowerCase().trim();
    if (normalized !== "") {
      set.add(normalized);
    }
  }
  return set;
}

/** equalFitSets reports whether two fit value lists name the same set, as agent/catalog does. */
function equalFitSets(a: string[], b: string[]): boolean {
  const left = normalizeFit(a);
  const right = normalizeFit(b);
  if (left.size !== right.size) {
    return false;
  }
  for (const value of left) {
    if (!right.has(value)) {
      return false;
    }
  }
  return true;
}

/**
 * isFitKey reports whether an attribute names a fit handle. agent/catalog
 * keeps only the category-specific cut labels — top-fit on shirts,
 * footwear-cut and shoe-collar-height on shoes — so any other attribute can
 * never be the cut, and a want naming one matches nothing rather than
 * accidentally comparing an unrelated attribute.
 */
function isFitKey(key: string): boolean {
  const lower = key.toLowerCase();
  return lower.includes("fit") || lower.includes("cut") || lower.includes("collar");
}

/** brandOf reads a product's first brand id, or empty. */
function brandOf(product: ProductRecord): string {
  const brands = product.brands;
  if (!Array.isArray(brands) || brands.length === 0) {
    return "";
  }
  const first = brands[0];
  if (typeof first !== "object" || first === null) {
    return "";
  }
  const id = (first as { id?: unknown }).id;
  return typeof id === "string" ? id : "";
}

/** categoryOf reads a product's category slug, or empty. */
function categoryOf(product: ProductRecord): string {
  const category = product.category;
  if (typeof category !== "object" || category === null) {
    return "";
  }
  const slug = (category as { slug?: unknown }).slug;
  return typeof slug === "string" ? slug : "";
}

/** fitValuesOf reads one fit handle's values from structured_attributes. */
function fitValuesOf(product: ProductRecord, attribute: string): string[] {
  const attributes = product.structured_attributes;
  if (typeof attributes !== "object" || attributes === null) {
    return [];
  }
  const values = (attributes as Record<string, unknown>)[attribute];
  if (!Array.isArray(values)) {
    return [];
  }
  return values.filter((value): value is string => typeof value === "string");
}

/** idOf reads a product's catalog id, or empty. */
function idOf(product: ProductRecord): string {
  return typeof product.id === "string" ? product.id : "";
}

/**
 * buildCatalog binds a transport to the match rule. It sends exactly the
 * query and limit the catalog takes, then keeps only the products that share
 * the want's cut, in catalog order. An incomplete want or an invalid search
 * matches nothing loudly instead of guessing.
 */
export function buildCatalog(transport: CatalogTransport): Catalog {
  return {
    async search(input: SearchInput): Promise<SearchResult> {
      const query = input.query.trim();
      if (query === "") {
        throw new CatalogError(0, CodeEmptyQuery, "the search names no query");
      }
      const limit = input.limit ?? DefaultLimit;
      if (!Number.isInteger(limit) || limit <= 0) {
        throw new CatalogError(0, CodeBadLimit, `the search limit ${String(limit)} is not a positive integer`);
      }
      const brand = input.brand_id.trim();
      const category = input.category.trim();
      const attribute = input.fit_attribute.trim();
      if (
        brand === "" ||
        category === "" ||
        attribute === "" ||
        !Array.isArray(input.fit_values) ||
        input.fit_values.length === 0
      ) {
        throw new CatalogError(
          0,
          CodeInvalidWant,
          "brand, category, fit attribute and fit values are all required",
        );
      }
      if (input.fit_values.some((value) => typeof value !== "string" || value.trim() === "")) {
        throw new CatalogError(0, CodeInvalidWant, "fit values carry no empty entry");
      }
      const products = await transport.search(query, limit);
      const excluded = new Set(input.exclude_skus ?? []);
      const matched: ProductRecord[] = [];
      for (const product of products) {
        if (excluded.has(idOf(product))) {
          continue;
        }
        if (brandOf(product) !== brand || categoryOf(product) !== category) {
          continue;
        }
        if (!isFitKey(attribute)) {
          continue;
        }
        if (!equalFitSets(fitValuesOf(product, attribute), input.fit_values)) {
          continue;
        }
        // structuredClone keeps every Channel3 field and adds none, so a
        // caller cannot narrow the record to fields the catalog did not send.
        matched.push(structuredClone(product));
      }
      return { products: matched };
    },
  };
}

/**
 * catalogFromEnv builds the deployed path's catalog from the deployment's
 * own environment. The key arrives from the platform's secret store and is
 * never read from a file; a deployment without one reports an unconfigured
 * search rather than answering from a recording.
 */
export function catalogFromEnv(env: Record<string, string | undefined> = process.env): Catalog {
  return buildCatalog(
    httpCatalogTransport({ url: env.CHANNEL3_CATALOG_URL, key: env.CHANNEL3_API_KEY }),
  );
}
