import { describe, expect, it } from 'vitest';
import { buildCurl } from './curl';
import { ALL_EVENT_IDS, coversAllEvents, normalizeEvents, parseEvents, toSubscribe } from './events';
import { formatPhone, initials, isHttpUrl, numberFromJid } from './format';

describe('buildCurl', () => {
  it('renders method, headers and body, escaping single quotes', () => {
    const out = buildCurl({
      method: 'post',
      url: 'http://h/send/text',
      headers: { apikey: 'k' },
      body: `{"text":"it's"}`,
    });
    expect(out).toBe(`curl -X POST 'http://h/send/text' \\\n  -H 'apikey: k' \\\n  -d '{"text":"it'\\''s"}'`);
  });
});

describe('events', () => {
  it('parses the stored comma list and detects full coverage', () => {
    expect(parseEvents('MESSAGE, CALL,,')).toEqual(['MESSAGE', 'CALL']);
    expect(parseEvents('')).toEqual([]);
    expect(coversAllEvents(ALL_EVENT_IDS)).toBe(true);
    expect(coversAllEvents(['MESSAGE'])).toBe(false);
  });

  it('normalizes to catalog order', () => {
    expect(normalizeEvents(['CALL', 'MESSAGE'])).toEqual(['MESSAGE', 'CALL']);
    expect(normalizeEvents(ALL_EVENT_IDS.slice().reverse())).toEqual(ALL_EVENT_IDS);
  });

  it('builds the subscribe payload', () => {
    expect(toSubscribe(['CALL'], true)).toEqual(['ALL']);
    expect(toSubscribe(['CALL', 'MESSAGE'], false)).toEqual(['MESSAGE', 'CALL']);
  });
});

describe('format', () => {
  it('extracts the number from a JID with a device suffix', () => {
    expect(numberFromJid('5511999990000:12@s.whatsapp.net')).toBe('5511999990000');
    expect(numberFromJid('')).toBe('');
  });

  it('formats Brazilian and foreign numbers', () => {
    expect(formatPhone('5511999990000')).toBe('+55 11 99999-0000');
    expect(formatPhone('551133334444')).toBe('+55 11 3333-4444');
    expect(formatPhone('14155550123')).toBe('+14155550123');
  });

  it('builds initials and validates urls', () => {
    expect(initials('minha-instancia')).toBe('MI');
    expect(initials('loja')).toBe('LO');
    expect(isHttpUrl('https://x.dev/hook')).toBe(true);
    expect(isHttpUrl('ftp://x')).toBe(false);
  });
});
