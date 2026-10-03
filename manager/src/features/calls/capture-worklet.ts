// Runs on the audio thread: hands the microphone's blocks (128 samples at the context's rate)
// to the page. It is built as a file of its own because the panel's Content-Security-Policy
// (script-src 'self') does not allow a worklet made from a Blob.

declare class AudioWorkletProcessor {
  readonly port: MessagePort;
}
declare function registerProcessor(name: string, processor: new () => AudioWorkletProcessor): void;

class PcmCapture extends AudioWorkletProcessor {
  process(inputs: Float32Array[][]): boolean {
    const channel = inputs[0]?.[0];
    // the block is reused by the browser: send a copy
    if (channel && channel.length > 0) this.port.postMessage(channel.slice());
    return true;
  }
}

registerProcessor('pcm-capture', PcmCapture);
