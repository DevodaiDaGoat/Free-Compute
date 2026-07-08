'use client';

import { useState } from 'react';

export default function BrowserApp() {
  const [url, setUrl] = useState('https://example.com');
  const [content, setContent] = useState('<h1>FreeCompute Browser</h1><p>Enter a URL and press Go to navigate.</p>');

  const navigate = async () => {
    try {
      const res = await fetch(url);
      const text = await res.text();
      setContent(text);
    } catch {
      setContent(`<h1>Error loading ${url}</h1><p>Could not fetch the URL. CORS may be blocking it.</p><p>Use the gateway proxy instead: /proxy/routeID/</p>`);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', background: '#0d1117' }}>
      <div style={{ display: 'flex', gap: 4, padding: 6, background: '#161b22', borderBottom: '1px solid #30363d' }}>
        <input
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && navigate()}
          style={{ flex: 1, padding: '6px 10px', background: '#0d1117', border: '1px solid #30363d', borderRadius: 4, color: '#ccc', fontSize: 12 }}
        />
        <button onClick={navigate} style={{ padding: '4px 12px', background: '#238636', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 12 }}>Go</button>
      </div>
      <iframe
        srcDoc={content}
        style={{ flex: 1, border: 'none', background: '#fff' }}
        title="browser-content"
        sandbox="allow-scripts"
      />
    </div>
  );
}
