import { NextRequest, NextResponse } from 'next/server';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

const ALLOWED_PROTOCOLS = new Set(['http:', 'https:']);

export async function GET(req: NextRequest) {
  const targetUrl = req.nextUrl.searchParams.get('url');
  if (!targetUrl) {
    return NextResponse.json({ error: 'Missing url parameter' }, { status: 400 });
  }

  let parsed: URL;
  try {
    parsed = new URL(targetUrl);
  } catch {
    return NextResponse.json({ error: 'Invalid URL' }, { status: 400 });
  }

  if (!ALLOWED_PROTOCOLS.has(parsed.protocol)) {
    return NextResponse.json({ error: 'Only HTTP/HTTPS URLs are supported' }, { status: 400 });
  }

  const fetchUrl = parsed.toString();

  try {
    const response = await fetch(fetchUrl, {
      headers: {
        'User-Agent': 'FreeCompute-Browser/1.0',
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
        'Accept-Language': 'en-US,en;q=0.5',
      },
      redirect: 'follow',
    });

    const contentType = response.headers.get('content-type') || 'text/plain';
    const body = await response.text();

    const isHtml =
      contentType.includes('text/html') ||
      /^<!doctype html/i.test(body) ||
      /<html/i.test(body);

    if (isHtml) {
      const baseUrl = `/api/browser/proxy?url=${encodeURIComponent(fetchUrl)}`;
      const processed = injectBase(body, baseUrl);
      const finalHtml = injectProxyScript(processed, baseUrl);
      return new NextResponse(finalHtml, {
        status: response.status,
        headers: { 'Content-Type': 'text/html; charset=utf-8' },
      });
    }

    return new NextResponse(body, {
      status: response.status,
      headers: { 'Content-Type': contentType },
    });
  } catch (err) {
    return NextResponse.json(
      { error: 'Fetch failed', detail: String(err) },
      { status: 502 }
    );
  }
}

function injectBase(html: string, base: string): string {
  const tag = `<base href="${base}">`;
  const headOpen = /<head[^>]*>/i.exec(html);
  if (headOpen) {
    const idx = headOpen.index + headOpen[0].length;
    return html.slice(0, idx) + tag + html.slice(idx);
  }
  const htmlOpen = /<html[^>]*>/i.exec(html);
  if (htmlOpen) {
    const idx = htmlOpen.index + htmlOpen[0].length;
    return html.slice(0, idx) + `<head>${tag}</head>` + html.slice(idx);
  }
  return `<head>${tag}</head>` + html;
}

function injectProxyScript(html: string, _baseUrl: string): string {
  const script = `<script src="/browser-proxy.js"><\/script>`;

  const headClose = /<\/head>/i.exec(html);
  if (headClose) {
    return html.slice(0, headClose.index) + script + html.slice(headClose.index);
  }
  return html + script;
}
