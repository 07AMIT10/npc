import { motion } from 'framer-motion';
import { useGameStore, TEAM_COLORS } from '../../store/gameStore';
import { useState, useEffect } from 'react';

function AnimatedScore({ value, color }: { value: number; color: string }) {
    const [displayValue, setDisplayValue] = useState(value);
    const [isAnimating, setIsAnimating] = useState(false);

    useEffect(() => {
        if (value !== displayValue) {
            setIsAnimating(true);
            const diff = value - displayValue;
            const steps = 15;
            const increment = diff / steps;
            let current = displayValue;
            let step = 0;

            const timer = setInterval(() => {
                step++;
                current += increment;
                if (step >= steps) {
                    setDisplayValue(value);
                    setIsAnimating(false);
                    clearInterval(timer);
                } else {
                    setDisplayValue(Math.round(current));
                }
            }, 30);

            return () => clearInterval(timer);
        }
    }, [value, displayValue]);

    return (
        <motion.span
            animate={isAnimating ? {
                scale: [1, 1.3, 1],
                textShadow: [`0 0 0px ${color}`, `0 0 20px ${color}`, `0 0 0px ${color}`]
            } : {}}
            transition={{ duration: 0.4 }}
            style={{
                color,
                fontWeight: 'bold',
                fontSize: '2rem',
                fontFamily: 'monospace'
            }}
        >
            {displayValue}
        </motion.span>
    );
}

export function Header() {
    const { teams, connected, slmProvider, brainProvider, running, setRunning, reset } = useGameStore();

    const redScore = teams.red?.score || 0;
    const blueScore = teams.blue?.score || 0;

    return (
        <header style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            padding: '1rem 2rem',
            background: 'linear-gradient(180deg, rgba(15,15,25,0.95) 0%, rgba(10,10,15,0.9) 100%)',
            borderBottom: '1px solid rgba(255,255,255,0.1)',
            backdropFilter: 'blur(10px)'
        }}>
            {/* Left: Title */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                <h1 style={{
                    margin: 0,
                    fontSize: '1.5rem',
                    fontWeight: 700,
                    background: 'linear-gradient(135deg, #a855f7 0%, #3b82f6 100%)',
                    WebkitBackgroundClip: 'text',
                    WebkitTextFillColor: 'transparent'
                }}>
                    🎮 NPC Arena
                </h1>
                <motion.div
                    animate={{ opacity: connected ? 1 : 0.5 }}
                    style={{
                        padding: '4px 12px',
                        borderRadius: '20px',
                        background: connected ? 'rgba(74, 222, 128, 0.2)' : 'rgba(239, 68, 68, 0.2)',
                        border: `1px solid ${connected ? '#4ade80' : '#ef4444'}`,
                        fontSize: '0.75rem',
                        color: connected ? '#4ade80' : '#ef4444'
                    }}
                >
                    {connected ? '● Connected' : '○ Disconnected'}
                </motion.div>
            </div>

            {/* Center: Scores */}
            <div style={{
                display: 'flex',
                alignItems: 'center',
                gap: '2rem',
                padding: '0.5rem 2rem',
                background: 'rgba(0,0,0,0.3)',
                borderRadius: '12px',
                border: '1px solid rgba(255,255,255,0.1)'
            }}>
                <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: '0.7rem', color: TEAM_COLORS.red.primary, marginBottom: '4px' }}>
                        TEAM RED
                    </div>
                    <AnimatedScore value={redScore} color={TEAM_COLORS.red.primary} />
                </div>

                <div style={{
                    fontSize: '1.5rem',
                    color: 'rgba(255,255,255,0.3)',
                    fontWeight: 300
                }}>
                    VS
                </div>

                <div style={{ textAlign: 'center' }}>
                    <div style={{ fontSize: '0.7rem', color: TEAM_COLORS.blue.primary, marginBottom: '4px' }}>
                        TEAM BLUE
                    </div>
                    <AnimatedScore value={blueScore} color={TEAM_COLORS.blue.primary} />
                </div>
            </div>

            {/* Right: Controls + Providers */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                <div style={{
                    fontSize: '0.7rem',
                    color: 'rgba(255,255,255,0.5)',
                    textAlign: 'right'
                }}>
                    <div>SLM: {slmProvider}</div>
                    <div>Brain: {brainProvider}</div>
                </div>

                <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <motion.button
                        whileHover={{ scale: 1.05 }}
                        whileTap={{ scale: 0.95 }}
                        onClick={() => setRunning(!running)}
                        style={{
                            padding: '8px 20px',
                            borderRadius: '8px',
                            border: 'none',
                            background: running
                                ? 'linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%)'
                                : 'linear-gradient(135deg, #4ade80 0%, #22c55e 100%)',
                            color: '#000',
                            fontWeight: 600,
                            cursor: 'pointer',
                            fontSize: '0.9rem'
                        }}
                    >
                        {running ? '⏸ Pause' : '▶ Start'}
                    </motion.button>

                    <motion.button
                        whileHover={{ scale: 1.05 }}
                        whileTap={{ scale: 0.95 }}
                        onClick={reset}
                        style={{
                            padding: '8px 16px',
                            borderRadius: '8px',
                            border: '1px solid rgba(255,255,255,0.2)',
                            background: 'transparent',
                            color: '#fff',
                            fontWeight: 500,
                            cursor: 'pointer',
                            fontSize: '0.9rem'
                        }}
                    >
                        🔄 Reset
                    </motion.button>
                </div>
            </div>
        </header>
    );
}
