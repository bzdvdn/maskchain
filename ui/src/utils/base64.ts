// @sk-task conversation-logging#T4.2: decodeBase64Utf8 decodes std base64 as UTF-8 (AC-005, AC-009)
export function decodeBase64Utf8(b64: string): string {
  const bin = atob(b64)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return new TextDecoder('utf-8').decode(bytes)
}
