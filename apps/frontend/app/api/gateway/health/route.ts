import { NextResponse } from 'next/server';

export const dynamic = 'force-dynamic';

const gatewayUrl = process.env.FREECOMPUTE_GATEWAY_URL ?? 'http://127.0.0.1:8080';

export async function GET() {
  const startedAt = Date.now();

  try {
    const response = await fetch(new URL('/healthz', gatewayUrl), {
      cache: 'no-store',
      signal: AbortSignal.timeout(1500),
    });

    return NextResponse.json({
      ok: response.ok,
      status: response.status,
      latencyMs: Date.now() - startedAt,
      gatewayUrl,
    });
  } catch (error) {
    return NextResponse.json(
      {
        ok: false,
        status: 0,
        latencyMs: Date.now() - startedAt,
        gatewayUrl,
        error: error instanceof Error ? error.message : 'Gateway unavailable',
      },
      { status: 200 },
    );
  }
}
