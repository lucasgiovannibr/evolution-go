/** Minimal Swagger 2.0 reader: just what the API explorer needs to list endpoints and seed request bodies. */

export interface SchemaProp {
  type?: string;
  example?: unknown;
  enum?: unknown[];
  $ref?: string;
  items?: { $ref?: string; type?: string };
}

export interface SwaggerSpec {
  info?: { title?: string; version?: string };
  paths: Record<string, Record<string, Operation>>;
  definitions?: Record<string, { properties?: Record<string, SchemaProp> }>;
}

export interface Operation {
  summary?: string;
  description?: string;
  tags?: string[];
  parameters?: Parameter[];
}

export interface Parameter {
  name: string;
  in: 'path' | 'query' | 'header' | 'body' | 'formData';
  required?: boolean;
  type?: string;
  description?: string;
  schema?: { $ref?: string };
}

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export interface Endpoint {
  key: string;
  method: Method;
  path: string;
  tag: string;
  summary: string;
  description: string;
  parameters: Parameter[];
}

const METHODS = ['get', 'post', 'put', 'patch', 'delete'];

export function parseSpec(spec: SwaggerSpec): Endpoint[] {
  const out: Endpoint[] = [];
  for (const [path, ops] of Object.entries(spec.paths ?? {})) {
    for (const [verb, op] of Object.entries(ops)) {
      if (!METHODS.includes(verb)) continue;
      const method = verb.toUpperCase() as Method;
      out.push({
        key: `${method} ${path}`,
        method,
        path,
        tag: op.tags?.[0] ?? 'Outros',
        summary: op.summary ?? '',
        description: op.description ?? '',
        parameters: op.parameters ?? [],
      });
    }
  }
  return out.sort((a, b) => a.tag.localeCompare(b.tag) || a.path.localeCompare(b.path) || a.method.localeCompare(b.method));
}

export function groupByTag(endpoints: Endpoint[]): [string, Endpoint[]][] {
  const map = new Map<string, Endpoint[]>();
  for (const e of endpoints) map.set(e.tag, [...(map.get(e.tag) ?? []), e]);
  return [...map.entries()].sort((a, b) => a[0].localeCompare(b[0]));
}

const defName = (ref: string) => ref.replace('#/definitions/', '');

/** Builds an example JSON body from a definition, honoring `example`/`enum` and guarding against cycles. */
export function exampleFromRef(spec: SwaggerSpec, ref: string | undefined, seen: Set<string> = new Set(), depth = 0): Record<string, unknown> | null {
  if (!ref || !ref.startsWith('#/definitions/') || depth > 5 || seen.has(ref)) return null;
  const def = spec.definitions?.[defName(ref)];
  if (!def?.properties) return null;
  const next = new Set(seen).add(ref);
  const out: Record<string, unknown> = {};
  for (const [key, p] of Object.entries(def.properties)) {
    if (p.example !== undefined) out[key] = p.example;
    else if (p.$ref) out[key] = exampleFromRef(spec, p.$ref, next, depth + 1);
    else if (p.type === 'array') {
      const item = p.items?.$ref ? exampleFromRef(spec, p.items.$ref, next, depth + 1) : null;
      out[key] = item ? [item] : [];
    } else if (p.enum?.length) out[key] = p.enum[0];
    else if (p.type === 'string') out[key] = '';
    else if (p.type === 'integer' || p.type === 'number') out[key] = 0;
    else if (p.type === 'boolean') out[key] = false;
    else out[key] = null;
  }
  return out;
}
