import { motion, AnimatePresence } from 'framer-motion';
import { useGameStore, TEAM_COLORS, CONFIG } from '../../store/gameStore';

export function Tooltip() {
    const { hoveredEntity, mousePos } = useGameStore();

    if (!hoveredEntity) return null;

    return (
        <AnimatePresence>
            <motion.div
                initial={{ opacity: 0, y: 10, scale: 0.95 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                exit={{ opacity: 0, scale: 0.95 }}
                style={{
                    position: 'fixed',
                    left: mousePos.x + 15,
                    top: mousePos.y + 15,
                    background: 'rgba(15, 15, 25, 0.95)',
                    border: '1px solid rgba(255, 255, 255, 0.2)',
                    borderRadius: '8px',
                    padding: '0.75rem',
                    minWidth: '150px',
                    backdropFilter: 'blur(10px)',
                    boxShadow: '0 10px 40px rgba(0, 0, 0, 0.5)',
                    zIndex: 1000,
                    pointerEvents: 'none'
                }}
            >
                {hoveredEntity.type === 'npc' && <NPCTooltip npc={hoveredEntity.data} />}
                {hoveredEntity.type === 'gate' && <GateTooltip gate={hoveredEntity.data} />}
            </motion.div>
        </AnimatePresence>
    );
}

function NPCTooltip({ npc }: { npc: any }) {
    const colors = TEAM_COLORS[npc.team as keyof typeof TEAM_COLORS];

    return (
        <>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '0.5rem' }}>
                <div style={{
                    width: '32px',
                    height: '32px',
                    borderRadius: '50%',
                    background: colors.bg,
                    border: `2px solid ${colors.primary}`,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '0.85rem',
                    fontWeight: 700,
                    color: colors.primary
                }}>
                    {npc.name[0]}
                </div>
                <div>
                    <div style={{ fontWeight: 600, color: '#fff' }}>{npc.name}</div>
                    <div style={{
                        fontSize: '0.7rem',
                        color: colors.primary,
                        textTransform: 'uppercase'
                    }}>
                        Team {npc.team}
                    </div>
                </div>
            </div>

            <div style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(3, 1fr)',
                gap: '0.5rem',
                marginBottom: '0.5rem'
            }}>
                <StatBadge icon="❤️" value={npc.hp} label="HP" />
                <StatBadge icon="⚡" value={Math.round(npc.energy)} label="Energy" />
                <StatBadge
                    icon={npc.state === 'moving' ? '💨' : npc.state === 'challenging' ? '🧩' : '💤'}
                    value={npc.state}
                    label="State"
                />
            </div>

            <div style={{
                fontSize: '0.7rem',
                color: 'rgba(255,255,255,0.5)',
                borderTop: '1px solid rgba(255,255,255,0.1)',
                paddingTop: '0.5rem'
            }}>
                📍 Position: [{Math.round(npc.x)}, {Math.round(npc.y)}]
            </div>

            {npc.thought && (
                <div style={{
                    fontSize: '0.75rem',
                    color: 'rgba(255,255,255,0.7)',
                    marginTop: '0.5rem',
                    fontStyle: 'italic'
                }}>
                    💭 "{npc.thought}"
                </div>
            )}
        </>
    );
}

function GateTooltip({ gate }: { gate: any }) {
    return (
        <>
            <div style={{ marginBottom: '0.5rem' }}>
                <div style={{
                    fontWeight: 600,
                    color: '#fff',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.5rem'
                }}>
                    <span style={{ fontSize: '1.2rem' }}>
                        {gate.unlocked ? '✅' : gate.requiresTeamwork ? '👥' : '🔒'}
                    </span>
                    {gate.id}
                </div>
                <div style={{
                    fontSize: '0.7rem',
                    color: gate.unlocked ? '#4ade80' : gate.requiresTeamwork ? '#a855f7' : '#fbbf24'
                }}>
                    {gate.unlocked ? 'Unlocked' : gate.requiresTeamwork ? 'Requires 2 Players' : 'Locked'}
                </div>
            </div>

            <div style={{
                fontSize: '0.7rem',
                color: 'rgba(255,255,255,0.5)'
            }}>
                📍 Position: [{gate.position[0]}, {gate.position[1]}]
            </div>

            {!gate.unlocked && (
                <div style={{
                    fontSize: '0.75rem',
                    color: 'rgba(255,255,255,0.6)',
                    marginTop: '0.5rem',
                    padding: '0.5rem',
                    background: 'rgba(255,255,255,0.05)',
                    borderRadius: '4px'
                }}>
                    💡 NPC must be within {CONFIG.INTERACTION_RANGE} units to challenge
                </div>
            )}
        </>
    );
}

function StatBadge({ icon, value }: { icon: string; value: string | number; label?: string }) {
    return (
        <div style={{
            background: 'rgba(255,255,255,0.05)',
            borderRadius: '4px',
            padding: '0.35rem',
            textAlign: 'center'
        }}>
            <div style={{ fontSize: '0.9rem' }}>{icon}</div>
            <div style={{ fontSize: '0.75rem', fontWeight: 600, color: '#fff' }}>{value}</div>
        </div>
    );
}

export function Minimap() {
    const { npcs, gates, zones } = useGameStore();
    const scale = 0.1;

    return (
        <div style={{
            position: 'absolute',
            bottom: '1rem',
            right: '1rem',
            width: CONFIG.WORLD_WIDTH * scale + 4,
            height: CONFIG.WORLD_HEIGHT * scale + 4,
            background: 'rgba(10, 10, 15, 0.9)',
            border: '2px solid rgba(255, 255, 255, 0.2)',
            borderRadius: '8px',
            overflow: 'hidden'
        }}>
            {/* Zones */}
            {Object.values(zones).map(zone => (
                <div
                    key={zone.id}
                    style={{
                        position: 'absolute',
                        left: zone.bounds.x * scale,
                        top: zone.bounds.y * scale,
                        width: zone.bounds.w * scale,
                        height: zone.bounds.h * scale,
                        background: zone.unlocked ? 'rgba(74, 222, 128, 0.1)' : 'rgba(55, 65, 81, 0.1)',
                        border: `1px solid ${zone.unlocked ? 'rgba(74, 222, 128, 0.3)' : 'rgba(55, 65, 81, 0.3)'}`
                    }}
                />
            ))}

            {/* Gates */}
            {Object.values(gates).map(gate => (
                <div
                    key={gate.id}
                    style={{
                        position: 'absolute',
                        left: gate.position[0] * scale - 3,
                        top: gate.position[1] * scale - 3,
                        width: 6,
                        height: 6,
                        borderRadius: '50%',
                        background: gate.unlocked ? '#4ade80' : gate.requiresTeamwork ? '#a855f7' : '#fbbf24'
                    }}
                />
            ))}

            {/* NPCs */}
            {npcs.map(npc => (
                <motion.div
                    key={npc.id}
                    animate={{
                        left: npc.x * scale - 4,
                        top: npc.y * scale - 4
                    }}
                    transition={{ type: 'spring', stiffness: 100 }}
                    style={{
                        position: 'absolute',
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        background: TEAM_COLORS[npc.team].primary,
                        boxShadow: `0 0 6px ${TEAM_COLORS[npc.team].primary}`
                    }}
                />
            ))}
        </div>
    );
}
