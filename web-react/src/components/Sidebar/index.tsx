import { motion, AnimatePresence } from 'framer-motion';
import { useGameStore, TEAM_COLORS } from '../../store/gameStore';
import type { NPC } from '../../store/gameStore';

function TeamPanel({ teamId, team }: { teamId: string; team: any }) {
    const { npcs } = useGameStore();
    const teamNPCs = npcs.filter(n => n.team === teamId);
    const colors = TEAM_COLORS[teamId as keyof typeof TEAM_COLORS];

    return (
        <motion.div
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            style={{
                background: colors.bg,
                border: `1px solid ${colors.primary}40`,
                borderRadius: '12px',
                padding: '1rem',
                marginBottom: '1rem'
            }}
        >
            <div style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: '0.75rem'
            }}>
                <span style={{
                    fontWeight: 600,
                    color: colors.primary,
                    fontSize: '0.9rem'
                }}>
                    {team?.name || `Team ${teamId}`}
                </span>
                <span style={{
                    fontWeight: 700,
                    fontSize: '1.1rem',
                    color: '#fff'
                }}>
                    {team?.score || 0}
                </span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                {teamNPCs.map(npc => (
                    <NPCStatus key={npc.id} npc={npc} color={colors.primary} />
                ))}
            </div>
        </motion.div>
    );
}

function NPCStatus({ npc, color }: { npc: NPC; color: string }) {
    return (
        <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: '0.5rem',
            padding: '0.5rem',
            background: 'rgba(0,0,0,0.2)',
            borderRadius: '8px'
        }}>
            <div style={{
                width: '28px',
                height: '28px',
                borderRadius: '50%',
                background: `${color}20`,
                border: `2px solid ${color}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '0.75rem',
                fontWeight: 600,
                color: color
            }}>
                {npc.name[0]}
            </div>
            <div style={{ flex: 1 }}>
                <div style={{
                    fontSize: '0.8rem',
                    fontWeight: 500,
                    color: '#fff'
                }}>
                    {npc.name}
                </div>
                <div style={{
                    fontSize: '0.65rem',
                    color: 'rgba(255,255,255,0.5)',
                    display: 'flex',
                    gap: '0.5rem'
                }}>
                    <span>❤️ {npc.hp}</span>
                    <span>⚡ {Math.round(npc.energy)}</span>
                    <span style={{
                        color: npc.state === 'moving' ? '#4ade80' :
                            npc.state === 'challenging' ? '#a855f7' : '#9ca3af'
                    }}>
                        {npc.state}
                    </span>
                </div>
            </div>
        </div>
    );
}

function LiveFeed() {
    const { feedItems } = useGameStore();

    return (
        <div style={{
            background: 'rgba(0,0,0,0.3)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: '12px',
            padding: '1rem',
            maxHeight: '250px',
            overflow: 'hidden'
        }}>
            <h3 style={{
                margin: '0 0 0.75rem 0',
                fontSize: '0.85rem',
                color: 'rgba(255,255,255,0.7)'
            }}>
                📡 Live Feed
            </h3>

            <div style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '0.5rem',
                maxHeight: '200px',
                overflowY: 'auto'
            }}>
                <AnimatePresence>
                    {feedItems.slice(0, 10).map((item) => (
                        <motion.div
                            key={item.id}
                            initial={{ opacity: 0, x: -20, height: 0 }}
                            animate={{ opacity: 1, x: 0, height: 'auto' }}
                            exit={{ opacity: 0, height: 0 }}
                            style={{
                                padding: '0.5rem',
                                background: 'rgba(255,255,255,0.05)',
                                borderRadius: '6px',
                                fontSize: '0.75rem',
                                borderLeft: `3px solid ${item.npc.color}`
                            }}
                        >
                            <span style={{ fontWeight: 600, color: item.npc.color }}>
                                {item.npc.name}
                            </span>
                            <span style={{ color: 'rgba(255,255,255,0.5)', marginLeft: '0.5rem' }}>
                                {item.action === 'taunt' ? '🗯️' : '💬'}
                            </span>
                            <div style={{ color: 'rgba(255,255,255,0.8)', marginTop: '0.25rem' }}>
                                "{item.text}"
                            </div>
                        </motion.div>
                    ))}
                </AnimatePresence>

                {feedItems.length === 0 && (
                    <div style={{
                        color: 'rgba(255,255,255,0.3)',
                        fontSize: '0.75rem',
                        textAlign: 'center',
                        padding: '1rem'
                    }}>
                        No activity yet...
                    </div>
                )}
            </div>
        </div>
    );
}

function Commentary() {
    const { commentary } = useGameStore();

    return (
        <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            style={{
                background: 'linear-gradient(135deg, rgba(168, 85, 247, 0.15) 0%, rgba(59, 130, 246, 0.15) 100%)',
                border: '1px solid rgba(168, 85, 247, 0.3)',
                borderRadius: '12px',
                padding: '1rem'
            }}
        >
            <h3 style={{
                margin: '0 0 0.5rem 0',
                fontSize: '0.85rem',
                color: '#a855f7'
            }}>
                🎙️ AI Commentary
            </h3>

            <AnimatePresence mode="wait">
                <motion.p
                    key={commentary}
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    style={{
                        margin: 0,
                        fontSize: '0.9rem',
                        color: 'rgba(255,255,255,0.9)',
                        fontStyle: 'italic'
                    }}
                >
                    {commentary || 'Waiting for action...'}
                </motion.p>
            </AnimatePresence>
        </motion.div>
    );
}

export function Sidebar() {
    const { teams } = useGameStore();

    return (
        <aside style={{
            width: '280px',
            padding: '1rem',
            display: 'flex',
            flexDirection: 'column',
            gap: '1rem',
            background: 'rgba(10,10,15,0.95)',
            borderLeft: '1px solid rgba(255,255,255,0.1)',
            overflowY: 'auto'
        }}>
            <TeamPanel teamId="red" team={teams.red} />
            <TeamPanel teamId="blue" team={teams.blue} />
            <LiveFeed />
            <Commentary />
        </aside>
    );
}
