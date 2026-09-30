import { describe, expect, it } from 'vitest';
import { exampleFromRef, groupByTag, parseSpec, type SwaggerSpec } from './swagger';

const spec: SwaggerSpec = {
  paths: {
    '/send/text': { post: { tags: ['Send'], summary: 'Send text', parameters: [{ name: 'body', in: 'body', schema: { $ref: '#/definitions/Text' } }] } },
    '/instance/all': { get: { tags: ['Instance'] }, options: { tags: ['Instance'] } },
  },
  definitions: {
    Text: { properties: { number: { type: 'string', example: '5511' }, delay: { type: 'integer' }, mode: { type: 'string', enum: ['a', 'b'] }, self: { $ref: '#/definitions/Text' } } },
  },
};

describe('swagger helpers', () => {
  it('lists supported methods only, sorted by tag then path', () => {
    const eps = parseSpec(spec);
    expect(eps.map((e) => e.key)).toEqual(['GET /instance/all', 'POST /send/text']);
    expect(groupByTag(eps).map(([t]) => t)).toEqual(['Instance', 'Send']);
  });

  it('seeds a body from examples, enums, types and survives cyclic refs', () => {
    expect(exampleFromRef(spec, '#/definitions/Text')).toEqual({ number: '5511', delay: 0, mode: 'a', self: null });
    expect(exampleFromRef(spec, undefined)).toBeNull();
  });
});
