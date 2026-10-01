import { describe, expect, it } from 'vitest';
import { defaultsOf, kindOf, KINDS, SAMPLE } from './send-catalog';

const NUMBER = '5531999990000';

describe('send catalog', () => {
  it('covers every /send endpoint of the API that has a body to fill', () => {
    expect(KINDS.map((k) => k.id).sort()).toEqual(
      ['button', 'carousel', 'contact', 'link', 'list', 'location', 'media', 'poll', 'status/media', 'status/text', 'sticker', 'text'].sort(),
    );
  });

  it('every default is either valid or only missing what the person has to type', () => {
    const defaults = defaultsOf();
    for (const k of KINDS) {
      const err = k.validate(defaults[k.id]);
      if (k.id === 'text' || k.id === 'status/text') expect(err).not.toBeNull();
      else expect(err, k.id).toBeNull();
    }
  });

  it('builds the exact bodies the server binds', () => {
    const d = defaultsOf();
    expect(kindOf('text').build(NUMBER, { text: 'oi' })).toEqual({ number: NUMBER, text: 'oi' });
    expect(kindOf('location').build(NUMBER, d.location)).toMatchObject({ number: NUMBER, latitude: -19.9319, longitude: -43.9378, name: 'Praça da Liberdade' });
    expect(kindOf('contact').build(NUMBER, { ...d.contact, phone: '+55 (11) 99999-0000' })).toEqual({
      number: NUMBER,
      vcard: { fullName: 'Contato de Teste', organization: 'Evolution GO', phone: '5511999990000' },
    });
    expect(kindOf('status/text').build(NUMBER, { text: 'oi' })).toEqual({ text: 'oi' });
  });

  it('polls take one option per line and the number of answers follows the mode', () => {
    const poll = kindOf('poll');
    const body = { question: 'Q', options: ' A \n\nB\nC ' };
    expect(poll.build(NUMBER, { ...body, mode: 'single' })).toEqual({ number: NUMBER, question: 'Q', options: ['A', 'B', 'C'], maxAnswer: 1 });
    expect(poll.build(NUMBER, { ...body, mode: 'multi' })).toMatchObject({ maxAnswer: 3 });
    expect(poll.validate({ question: 'Q', options: 'A', mode: 'single' })).toMatch(/2 a 12/);
  });

  it('media only sends the fields that apply to its type', () => {
    const media = kindOf('media');
    const base = { url: SAMPLE.audio, caption: 'c', filename: 'f.pdf', viewOnce: 'yes' };
    expect(media.build(NUMBER, { ...base, type: 'audio' })).toEqual({ number: NUMBER, type: 'audio', url: SAMPLE.audio, viewOnce: true });
    expect(media.build(NUMBER, { ...base, type: 'document' })).toEqual({ number: NUMBER, type: 'document', url: SAMPLE.audio, caption: 'c', filename: 'f.pdf' });
    expect(media.build(NUMBER, { ...base, type: 'image', viewOnce: 'no' })).toEqual({ number: NUMBER, type: 'image', url: SAMPLE.audio, caption: 'c' });
  });

  it('the sample URL follows the media type until the person types their own', () => {
    const media = kindOf('media');
    const d = defaultsOf().media;
    expect(media.patch?.('type', 'video', d)?.url).toBe(SAMPLE.video);
    expect(media.patch?.('type', 'video', { ...d, url: 'https://mine.example/x.mp4' })?.url).toBe('https://mine.example/x.mp4');
  });

  it('rejects what the server would refuse', () => {
    expect(kindOf('location').validate({ name: 'x', latitude: '91', longitude: '0' })).toMatch(/latitude/);
    expect(kindOf('media').validate({ url: 'ftp://x' })).toMatch(/http/);
    expect(kindOf('contact').validate({ fullName: 'A', phone: '12' })).toMatch(/telefone/);
  });

  it('interactive kinds are built from presets for the right endpoint', () => {
    const d = defaultsOf();
    expect(kindOf('button').build(NUMBER, d.button)).toMatchObject({ number: NUMBER, buttons: [{ type: 'reply' }] });
    expect(kindOf('list').build(NUMBER, d.list)).toHaveProperty('sections');
    expect(kindOf('carousel').build(NUMBER, d.carousel)).toHaveProperty('cards');
  });

  it('the status has no recipient and asks for confirmation', () => {
    for (const id of ['status/text', 'status/media'] as const) {
      expect(kindOf(id).recipient).toBe(false);
      expect(kindOf(id).confirm).toBeTruthy();
    }
  });
});
