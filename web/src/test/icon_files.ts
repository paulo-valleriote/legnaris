import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

/** The directory the static host serves; every icon URL resolves beneath it. */
export const publicDir = path.resolve(here, '../../public');

/** The file behind a root-relative URL such as `/icons/favicon.svg`. */
export function publicFile(url: string): string {
  return path.join(publicDir, url);
}

/** The pixel size of a PNG read from its IHDR chunk, or null when the bytes are not a PNG. */
export function pngSize(file: string): { width: number; height: number } | null {
  const bytes = readFileSync(file);
  const signature = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
  if (bytes.length < 24 || !signature.every((byte, index) => bytes[index] === byte)) {
    return null;
  }
  return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

/** The number of images in an ICO container, or -1 when the header is not an icon directory. */
export function icoImageCount(file: string): number {
  const bytes = readFileSync(file);
  if (bytes.length < 6 || bytes.readUInt16LE(0) !== 0 || bytes.readUInt16LE(2) !== 1) {
    return -1;
  }
  return bytes.readUInt16LE(4);
}

/** The edge length declared by a `WxH` sizes value when it is square, otherwise null. */
export function declaredSize(sizes: string | undefined): number | null {
  const match = /^(\d+)x(\d+)$/.exec(sizes ?? '');
  return match && match[1] === match[2] ? Number(match[1]) : null;
}
