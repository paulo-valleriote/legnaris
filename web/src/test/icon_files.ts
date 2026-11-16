import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

export const publicDir = path.resolve(here, '../../public');

export function publicFile(url: string): string {
  return path.join(publicDir, url);
}

export function pngSize(file: string): { width: number; height: number } | null {
  const bytes = readFileSync(file);
  const signature = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
  if (bytes.length < 24 || !signature.every((byte, index) => bytes[index] === byte)) {
    return null;
  }
  return { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20) };
}

export function icoImageCount(file: string): number {
  const bytes = readFileSync(file);
  if (bytes.length < 6 || bytes.readUInt16LE(0) !== 0 || bytes.readUInt16LE(2) !== 1) {
    return -1;
  }
  return bytes.readUInt16LE(4);
}

export function declaredSize(sizes: string | undefined): number | null {
  const match = /^(\d+)x(\d+)$/.exec(sizes ?? '');
  return match && match[1] === match[2] ? Number(match[1]) : null;
}
