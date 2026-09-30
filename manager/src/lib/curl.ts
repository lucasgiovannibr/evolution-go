export interface CurlInput {
  method: string;
  url: string;
  headers?: Record<string, string>;
  body?: string;
}

const quote = (s: string) => `'${s.replace(/'/g, "'\\''")}'`;

export function buildCurl({ method, url, headers = {}, body }: CurlInput): string {
  const parts = [`curl -X ${method.toUpperCase()} ${quote(url)}`];
  for (const [k, v] of Object.entries(headers)) parts.push(`  -H ${quote(`${k}: ${v}`)}`);
  if (body) parts.push(`  -d ${quote(body)}`);
  return parts.join(' \\\n');
}
