'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { CreditCard, Copy, Check, Zap, Star, Shield, RefreshCw, TrendingUp } from 'lucide-react';
import { apiFetch, getGatewayUrl, getTokens, getUser } from '../../boot/BootSequence';

interface TxRow {
  id: string;
  kind: 'earn' | 'spend';
  amount: number;
  reason: string;
  ts: string;
}

interface CryptoOption {
  symbol: string;
  name: string;
  address: string;
  rate: number;
  color: string;
  logo: string;
}

const CRYPTO: CryptoOption[] = [
  { symbol: 'BTC',  name: 'Bitcoin',  address: 'bc1qfreecompute000000000000000000000000demo', rate: 500,   color: '#f7931a', logo: '₿' },
  { symbol: 'ETH',  name: 'Ethereum', address: '0xFreeCompute00000000000000000000000000DEMO', rate: 250,   color: '#627eea', logo: 'Ξ' },
  { symbol: 'SOL',  name: 'Solana',   address: 'FreeComputeSOLDemoAddress111111111111111111',  rate: 50,    color: '#9945ff', logo: '◎' },
  { symbol: 'USDC', name: 'USDC',     address: '0xFreeComputeUSDC0000000000000000000000DEMO', rate: 100,   color: '#2775ca', logo: '$' },
];

const PLANS = [
  { name: 'Starter',    credits: 500,   price: 5,   usd: '$5',  tag: null,          perHour: 2, desc: '~250 CPU-hours' },
  { name: 'Standard',   credits: 2500,  price: 20,  usd: '$20', tag: 'Popular',     perHour: 2, desc: '~1,250 CPU-hours' },
  { name: 'Power',      credits: 7000,  price: 50,  usd: '$50', tag: 'Best Value',  perHour: 2, desc: '~3,500 CPU-hours' },
  { name: 'Community',  credits: 50000, price: 0,   usd: 'Free', tag: 'Donate',     perHour: 0, desc: 'Contribute your compute' },
];

const WAYS_TO_EARN = [
  { icon: <Zap size={14} />, label: 'Host Compute',    credits: '+50/hr',  desc: 'Donate CPU/GPU time' },
  { icon: <Star size={14} />, label: 'Bug Reports',     credits: '+25',     desc: 'Accepted reports' },
  { icon: <Shield size={14} />,label: 'Beta Testing',   credits: '+100',    desc: 'Per session tested' },
  { icon: <TrendingUp size={14} />, label: 'Referrals', credits: '+200',    desc: 'Per active user referred' },
];

const FAKE_TXS: TxRow[] = [
  { id: '1', kind: 'earn',  amount: 100,  reason: 'Hosting session — vm-001',  ts: '2026-07-11 21:40' },
  { id: '2', kind: 'spend', amount: -20,  reason: 'Desktop session (1h)',       ts: '2026-07-11 20:10' },
  { id: '3', kind: 'earn',  amount: 25,   reason: 'Bug report accepted',        ts: '2026-07-10 14:22' },
  { id: '4', kind: 'spend', amount: -40,  reason: 'Gaming session — GPU (2h)',  ts: '2026-07-09 18:05' },
];

function CopyBtn({ text }: { text: string }) {
  const [ok, setOk] = useState(false);
  const mountedRef = useRef(true);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => {
    mountedRef.current = false;
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);
  const copy = () => {
    navigator.clipboard.writeText(text).then(() => {
      if (!mountedRef.current) return;
      setOk(true);
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => {
        if (mountedRef.current) setOk(false);
      }, 2000);
    }).catch(() => { /* clipboard denied — silently ignore */ });
  };
  return (
    <button onClick={copy} style={{ background: 'none', border: 'none', cursor: 'pointer', color: ok ? '#3fb950' : '#6e7681', padding: 4, display: 'flex', alignItems: 'center' }}>
      {ok ? <Check size={13} /> : <Copy size={13} />}
    </button>
  );
}

export default function CreditsApp() {
  const [tab, setTab] = useState<'buy' | 'crypto' | 'earn' | 'history'>('buy');
  const [selectedCrypto, setSelectedCrypto] = useState<CryptoOption>(CRYPTO[0]);
  const [credits, setCredits] = useState<number | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const user = getUser();
  const mountedRef = useRef(true);

  useEffect(() => () => { mountedRef.current = false; }, []);

  const fetchCredits = useCallback(async () => {
    if (mountedRef.current) setRefreshing(true);
    try {
      const data = await apiFetch('/auth/profile');
      if (mountedRef.current && data && typeof data.credits === 'number') setCredits(data.credits);
    } catch {
      // apiFetch throws on 4xx. The fallback must also attach the Bearer
      // token — /auth/profile is authenticated, so an unauthenticated retry
      // just returns 401 again and the credits stay at "—".
      try {
        const token = getTokens()?.accessToken;
        const headers: Record<string, string> = { 'Accept': 'application/json' };
        if (token) headers['Authorization'] = `Bearer ${token}`;
        const r = await fetch(`${getGatewayUrl()}/auth/profile`, { headers });
        if (r.ok) {
          const data = await r.json().catch(() => null);
          if (mountedRef.current && data && typeof data.credits === 'number') setCredits(data.credits);
        }
      } catch { /* ignore */ }
    } finally {
      if (mountedRef.current) setRefreshing(false);
    }
  }, []);

  useEffect(() => { fetchCredits(); }, [fetchCredits]);

  const TAB_STYLE = (active: boolean) => ({
    padding: '6px 16px',
    borderRadius: 6,
    border: 'none',
    background: active ? 'rgba(88,166,255,0.15)' : 'transparent',
    color: active ? '#58a6ff' : '#6e7681',
    cursor: 'pointer',
    fontSize: 12,
    fontWeight: 700,
    borderBottom: active ? '2px solid #58a6ff' : '2px solid transparent',
  });

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', background: '#0a0f1e', color: '#c9d1d9', fontFamily: 'system-ui, sans-serif', overflow: 'hidden' }}>
      {/* Header */}
      <div style={{ padding: '12px 18px', background: 'rgba(22,27,34,0.9)', borderBottom: '1px solid rgba(48,54,61,0.5)', display: 'flex', alignItems: 'center', gap: 12, flexShrink: 0 }}>
        <CreditCard size={17} color="#d2a8ff" />
        <span style={{ fontSize: 14, fontWeight: 700, color: '#e6edf3' }}>Credits &amp; Billing</span>
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 10 }}>
          <div style={{ padding: '6px 14px', background: 'rgba(88,166,255,0.1)', border: '1px solid rgba(88,166,255,0.2)', borderRadius: 8, fontSize: 13 }}>
            <span style={{ color: '#6e7681', fontSize: 11 }}>Balance: </span>
            <span style={{ fontWeight: 700, color: '#e6edf3', fontSize: 16 }}>
              {credits !== null ? credits.toLocaleString() : '—'}
            </span>
            <span style={{ color: '#6e7681', fontSize: 11 }}> credits</span>
          </div>
          <button onClick={fetchCredits} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#6e7681', padding: 4, display: 'flex', alignItems: 'center' }}>
            <RefreshCw size={13} className={refreshing ? 'spin' : ''} />
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 2, padding: '6px 12px', borderBottom: '1px solid rgba(48,54,61,0.4)', background: 'rgba(13,17,23,0.5)', flexShrink: 0 }}>
        {(['buy', 'crypto', 'earn', 'history'] as const).map((t) => (
          <button key={t} onClick={() => setTab(t)} style={TAB_STYLE(tab === t)}>
            {t === 'buy' ? 'Buy Credits' : t === 'crypto' ? 'Crypto' : t === 'earn' ? 'Earn Free' : 'History'}
          </button>
        ))}
      </div>

      <div style={{ flex: 1, overflowY: 'auto', padding: 16 }}>

        {tab === 'buy' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ fontSize: 12, color: '#6e7681', marginBottom: 4 }}>
              1 credit = 1 CPU-minute of basic compute · GPU sessions cost 2-4x credits
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 10 }}>
              {PLANS.map((plan) => (
                <div key={plan.name} style={{ padding: '16px', borderRadius: 12, border: plan.tag === 'Popular' ? '1px solid rgba(88,166,255,0.4)' : plan.tag === 'Best Value' ? '1px solid rgba(63,185,80,0.4)' : '1px solid rgba(255,255,255,0.08)', background: plan.tag === 'Popular' ? 'rgba(88,166,255,0.06)' : plan.tag === 'Best Value' ? 'rgba(63,185,80,0.06)' : 'rgba(255,255,255,0.03)', position: 'relative', cursor: plan.price > 0 ? 'pointer' : 'default' }}>
                  {plan.tag && (
                    <div style={{ position: 'absolute', top: -10, right: 14, fontSize: 10, fontWeight: 700, padding: '2px 8px', borderRadius: 20, background: plan.tag === 'Popular' ? '#58a6ff' : '#238636', color: '#fff', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                      {plan.tag}
                    </div>
                  )}
                  <div style={{ fontSize: 13, fontWeight: 700, color: '#e6edf3', marginBottom: 6 }}>{plan.name}</div>
                  <div style={{ fontSize: 22, fontWeight: 800, color: plan.price === 0 ? '#3fb950' : '#e6edf3', letterSpacing: '-0.02em', marginBottom: 4 }}>{plan.usd}</div>
                  <div style={{ fontSize: 13, color: '#58a6ff', fontWeight: 700, marginBottom: 4 }}>{plan.credits > 0 ? `${plan.credits.toLocaleString()} credits` : 'Earn credits'}</div>
                  <div style={{ fontSize: 11, color: '#6e7681' }}>{plan.desc}</div>
                  {plan.price > 0 && (
                    <button style={{ marginTop: 12, width: '100%', padding: '8px', borderRadius: 8, background: plan.tag === 'Popular' ? '#1f6feb' : plan.tag === 'Best Value' ? '#238636' : 'rgba(255,255,255,0.08)', border: 'none', color: '#fff', fontSize: 12, fontWeight: 700, cursor: 'pointer' }}>
                      Buy Now
                    </button>
                  )}
                  {plan.price === 0 && (
                    <button
                      onClick={() => setTab('earn')}
                      style={{ marginTop: 12, width: '100%', padding: '8px', borderRadius: 8, background: 'rgba(63,185,80,0.15)', border: '1px solid rgba(63,185,80,0.3)', color: '#3fb950', fontSize: 12, fontWeight: 700, cursor: 'pointer' }}>
                      Start Hosting
                    </button>
                  )}
                </div>
              ))}
            </div>
            <div style={{ fontSize: 11, color: '#484f58', textAlign: 'center', marginTop: 4 }}>
              Payments powered by Stripe (coming soon) · All purchases non-refundable
            </div>
          </div>
        )}

        {tab === 'crypto' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ fontSize: 12, color: '#6e7681' }}>Send crypto to receive credits instantly. <strong style={{ color: '#c9d1d9' }}>Minimum: 1 USD equivalent.</strong> Credits arrive after 1 on-chain confirmation.</div>

            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {CRYPTO.map((c) => (
                <button key={c.symbol} onClick={() => setSelectedCrypto(c)} style={{ padding: '8px 14px', borderRadius: 8, border: `1px solid ${selectedCrypto.symbol === c.symbol ? c.color : 'rgba(255,255,255,0.08)'}`, background: selectedCrypto.symbol === c.symbol ? `${c.color}18` : 'rgba(255,255,255,0.03)', color: selectedCrypto.symbol === c.symbol ? c.color : '#8b949e', cursor: 'pointer', fontSize: 14, fontWeight: 700, display: 'flex', alignItems: 'center', gap: 6 }}>
                  <span>{c.logo}</span>
                  {c.symbol}
                </button>
              ))}
            </div>

            <div style={{ padding: '18px', borderRadius: 12, border: `1px solid ${selectedCrypto.color}30`, background: `${selectedCrypto.color}08` }}>
              <div style={{ fontSize: 11, color: '#6e7681', marginBottom: 6, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                {selectedCrypto.name} ({selectedCrypto.symbol}) Deposit Address
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, background: 'rgba(0,0,0,0.4)', borderRadius: 8, padding: '10px 12px', border: '1px solid rgba(255,255,255,0.06)' }}>
                <span style={{ fontSize: 11, fontFamily: 'monospace', color: '#c9d1d9', flex: 1, wordBreak: 'break-all' }}>{selectedCrypto.address}</span>
                <CopyBtn text={selectedCrypto.address} />
              </div>
              <div style={{ marginTop: 12, fontSize: 13, color: '#8b949e' }}>
                Exchange rate: <strong style={{ color: '#e6edf3' }}>{selectedCrypto.rate} credits</strong> per $1 USD equivalent
              </div>
              <div style={{ marginTop: 6, fontSize: 11, color: '#484f58' }}>
                This is a demo address. Real payment processing is coming soon.
              </div>
            </div>
          </div>
        )}

        {tab === 'earn' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            <div style={{ fontSize: 13, color: '#8b949e', lineHeight: 1.65 }}>
              Earn credits by contributing to the FreeCompute network. No purchase needed.
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {WAYS_TO_EARN.map((w) => (
                <div key={w.label} style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '14px 16px', borderRadius: 10, border: '1px solid rgba(255,255,255,0.07)', background: 'rgba(255,255,255,0.03)' }}>
                  <span style={{ color: '#58a6ff', width: 28, height: 28, display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'rgba(88,166,255,0.1)', borderRadius: 8, flexShrink: 0 }}>{w.icon}</span>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 13, fontWeight: 700, color: '#e6edf3' }}>{w.label}</div>
                    <div style={{ fontSize: 11, color: '#6e7681', marginTop: 2 }}>{w.desc}</div>
                  </div>
                  <span style={{ fontSize: 14, fontWeight: 800, color: '#3fb950' }}>{w.credits}</span>
                </div>
              ))}
            </div>
            <div style={{ padding: '14px 16px', borderRadius: 10, border: '1px solid rgba(88,166,255,0.2)', background: 'rgba(88,166,255,0.06)', fontSize: 12, color: '#8b949e', lineHeight: 1.65 }}>
              <strong style={{ color: '#58a6ff' }}>Your hardware matters:</strong> Even integrated graphics (Intel Iris Xe, AMD Vega) earns credits for software-encoded desktop sessions.
              Discrete GPUs unlock GPU-accelerated gaming sessions at higher earn rates.
            </div>
          </div>
        )}

        {tab === 'history' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
            {FAKE_TXS.map((tx) => (
              <div key={tx.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderBottom: '1px solid rgba(255,255,255,0.05)' }}>
                <span style={{ fontSize: 14, fontWeight: 700, color: tx.kind === 'earn' ? '#3fb950' : '#f85149', minWidth: 56, textAlign: 'right' }}>
                  {tx.kind === 'earn' ? '+' : ''}{tx.amount}
                </span>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 12, color: '#c9d1d9' }}>{tx.reason}</div>
                  <div style={{ fontSize: 10, color: '#484f58', marginTop: 2 }}>{tx.ts}</div>
                </div>
              </div>
            ))}
            {FAKE_TXS.length === 0 && (
              <div style={{ textAlign: 'center', color: '#484f58', fontSize: 13, padding: 32 }}>No transactions yet</div>
            )}
          </div>
        )}

      </div>
    </div>
  );
}
