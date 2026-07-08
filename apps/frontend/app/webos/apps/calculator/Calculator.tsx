'use client';

import { useState } from 'react';

export default function Calculator() {
  const [display, setDisplay] = useState('0');
  const [op, setOp] = useState<string | null>(null);
  const [prev, setPrev] = useState<number | null>(null);
  const [reset, setReset] = useState(false);

  const press = (val: string) => {
    if ('0123456789'.includes(val)) {
      if (display === '0' || reset) {
        setDisplay(val);
        setReset(false);
      } else {
        setDisplay(display + val);
      }
    }
  };

  const opPress = (opVal: string) => {
    setPrev(parseFloat(display));
    setOp(opVal);
    setReset(true);
  };

  const equals = () => {
    if (prev === null || !op) return;
    const cur = parseFloat(display);
    let result = 0;
    switch (op) {
      case '+': result = prev + cur; break;
      case '-': result = prev - cur; break;
      case '*': result = prev * cur; break;
      case '/': result = cur !== 0 ? prev / cur : 0; break;
    }
    setDisplay(String(result));
    setPrev(null);
    setOp(null);
    setReset(true);
  };

  const clear = () => { setDisplay('0'); setPrev(null); setOp(null); };

  return (
    <div style={{ padding: 16, height: '100%', display: 'flex', flexDirection: 'column', fontFamily: 'monospace' }}>
      <div style={{ background: '#0a0a0a', padding: '12px 16px', borderRadius: 6, marginBottom: 12, textAlign: 'right', fontSize: 24, color: '#fff' }}>{display}</div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 6, flex: 1 }}>
        {['7','8','9','/','4','5','6','*','1','2','3','-','0','.','=','+'].map((b) => (
          <button
            key={b}
            onClick={() => { if (b === '=') equals(); else if ('+-*/'.includes(b)) opPress(b); else if (b === '.') press('.'); else press(b); }}
            style={{
              padding: 8, background: '+-*/'.includes(b) ? '#2a2a5a' : '#111', border: '1px solid #2a2a4a',
              borderRadius: 4, color: '#ccc', cursor: 'pointer', fontSize: 16,
            }}
          >
            {b}
          </button>
        ))}
      </div>
      <button onClick={clear} style={{ marginTop: 6, padding: 8, background: '#331111', border: '1px solid #552222', borderRadius: 4, color: '#f66', cursor: 'pointer' }}>C</button>
    </div>
  );
}
