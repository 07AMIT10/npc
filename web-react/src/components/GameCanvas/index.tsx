import { useRef, useEffect, useCallback } from 'react';
import { useGameStore, CONFIG, TEAM_COLORS } from '../../store/gameStore';

export function GameCanvas() {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const {
        npcs,
        zones,
        gates,
        particles,
        running,
        tick,
        setTick,
        updateNPC,
        updateParticles,
        addParticle,
        setHoveredEntity,
        setMousePos
    } = useGameStore();

    // Update NPC positions
    const updateNPCs = useCallback(() => {
        npcs.forEach(npc => {
            const dx = npc.targetX - npc.x;
            const dy = npc.targetY - npc.y;
            const dist = Math.sqrt(dx * dx + dy * dy);

            if (dist > 2) {
                const speed = CONFIG.NPC_SPEED;
                const vx = (dx / dist) * speed;
                const vy = (dy / dist) * speed;

                updateNPC(npc.id, {
                    x: npc.x + vx,
                    y: npc.y + vy,
                    angle: Math.atan2(dy, dx)
                });

                // Add trail particle occasionally
                if (tick % 5 === 0) {
                    addParticle({
                        x: npc.x,
                        y: npc.y,
                        vx: (Math.random() - 0.5) * 0.3,
                        vy: (Math.random() - 0.5) * 0.3,
                        life: 20,
                        maxLife: 20,
                        color: TEAM_COLORS[npc.team].primary + '40',
                        size: 3 + Math.random() * 2,
                        type: 'trail'
                    });
                }
            } else if (npc.state === 'moving') {
                updateNPC(npc.id, { state: 'idle' });
            }
        });
    }, [npcs, tick, updateNPC, addParticle]);

    // Render function
    const render = useCallback(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        // Clear
        ctx.fillStyle = '#0a0a0f';
        ctx.fillRect(0, 0, CONFIG.WORLD_WIDTH, CONFIG.WORLD_HEIGHT);

        // Draw zones
        Object.values(zones).forEach(zone => {
            const b = zone.bounds;
            ctx.fillStyle = zone.unlocked ? 'rgba(30, 30, 40, 0.8)' : 'rgba(20, 20, 30, 0.5)';
            ctx.fillRect(b.x, b.y, b.w, b.h);

            // Zone border
            ctx.strokeStyle = zone.unlocked ? '#4ade80' : '#374151';
            ctx.lineWidth = 2;
            ctx.strokeRect(b.x + 1, b.y + 1, b.w - 2, b.h - 2);

            // Zone name
            ctx.font = '14px "Inter", sans-serif';
            ctx.fillStyle = zone.unlocked ? '#9ca3af' : '#4b5563';
            ctx.fillText(zone.name, b.x + 10, b.y + 24);
        });

        // Draw gates
        Object.values(gates).forEach(gate => {
            const [x, y] = gate.position;

            // Gate glow
            const gradient = ctx.createRadialGradient(x, y, 0, x, y, CONFIG.GATE_RADIUS * 1.5);
            if (gate.unlocked) {
                gradient.addColorStop(0, 'rgba(74, 222, 128, 0.3)');
                gradient.addColorStop(1, 'rgba(74, 222, 128, 0)');
            } else if (gate.requiresTeamwork) {
                gradient.addColorStop(0, 'rgba(168, 85, 247, 0.3)');
                gradient.addColorStop(1, 'rgba(168, 85, 247, 0)');
            } else {
                gradient.addColorStop(0, 'rgba(251, 191, 36, 0.3)');
                gradient.addColorStop(1, 'rgba(251, 191, 36, 0)');
            }
            ctx.fillStyle = gradient;
            ctx.beginPath();
            ctx.arc(x, y, CONFIG.GATE_RADIUS * 1.5, 0, Math.PI * 2);
            ctx.fill();

            // Gate circle
            ctx.beginPath();
            ctx.arc(x, y, CONFIG.GATE_RADIUS, 0, Math.PI * 2);
            ctx.fillStyle = gate.unlocked ? '#1a2e1a' : '#1a1a2e';
            ctx.fill();
            ctx.strokeStyle = gate.unlocked ? '#4ade80' : gate.requiresTeamwork ? '#a855f7' : '#fbbf24';
            ctx.lineWidth = 3;
            ctx.stroke();

            // Gate icon
            ctx.font = '16px sans-serif';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.fillStyle = gate.unlocked ? '#4ade80' : '#fff';
            ctx.fillText(gate.unlocked ? '✓' : gate.requiresTeamwork ? '👥' : '🔒', x, y);
        });

        // Draw particles
        particles.forEach(p => {
            const alpha = p.life / p.maxLife;
            ctx.beginPath();
            ctx.arc(p.x, p.y, p.size * alpha, 0, Math.PI * 2);
            ctx.fillStyle = p.color;
            ctx.globalAlpha = alpha;
            ctx.fill();
            ctx.globalAlpha = 1;
        });

        // Draw NPCs
        npcs.forEach(npc => {
            const teamColor = TEAM_COLORS[npc.team];

            // NPC glow
            const glowGradient = ctx.createRadialGradient(npc.x, npc.y, 0, npc.x, npc.y, CONFIG.NPC_RADIUS * 2);
            glowGradient.addColorStop(0, teamColor.glow);
            glowGradient.addColorStop(1, 'rgba(0,0,0,0)');
            ctx.fillStyle = glowGradient;
            ctx.beginPath();
            ctx.arc(npc.x, npc.y, CONFIG.NPC_RADIUS * 2, 0, Math.PI * 2);
            ctx.fill();

            // NPC body
            ctx.beginPath();
            ctx.arc(npc.x, npc.y, CONFIG.NPC_RADIUS, 0, Math.PI * 2);
            ctx.fillStyle = teamColor.bg;
            ctx.fill();
            ctx.strokeStyle = teamColor.primary;
            ctx.lineWidth = 3;
            ctx.stroke();

            // Direction indicator
            ctx.beginPath();
            ctx.moveTo(npc.x, npc.y);
            ctx.lineTo(
                npc.x + Math.cos(npc.angle) * CONFIG.NPC_RADIUS,
                npc.y + Math.sin(npc.angle) * CONFIG.NPC_RADIUS
            );
            ctx.strokeStyle = teamColor.primary;
            ctx.lineWidth = 2;
            ctx.stroke();

            // NPC name
            ctx.font = 'bold 12px "Inter", sans-serif';
            ctx.textAlign = 'center';
            ctx.fillStyle = '#fff';
            ctx.fillText(npc.name, npc.x, npc.y - CONFIG.NPC_RADIUS - 8);

            // Thought bubble
            if (npc.thought) {
                ctx.font = '10px "Inter", sans-serif';
                ctx.fillStyle = 'rgba(255,255,255,0.7)';
                const thought = npc.thought.length > 25 ? npc.thought.slice(0, 25) + '...' : npc.thought;
                ctx.fillText(thought, npc.x, npc.y + CONFIG.NPC_RADIUS + 16);
            }

            // State indicator
            if (npc.state === 'challenging') {
                ctx.font = '14px sans-serif';
                ctx.fillText('🧩', npc.x + CONFIG.NPC_RADIUS, npc.y - CONFIG.NPC_RADIUS);
            } else if (npc.state === 'moving') {
                ctx.font = '14px sans-serif';
                ctx.fillText('💨', npc.x + CONFIG.NPC_RADIUS, npc.y - CONFIG.NPC_RADIUS);
            }
        });

        ctx.textAlign = 'left';
        ctx.textBaseline = 'alphabetic';
    }, [zones, gates, particles, npcs]);

    // Handle mouse move for hover
    const handleMouseMove = useCallback((e: React.MouseEvent<HTMLCanvasElement>) => {
        const canvas = canvasRef.current;
        if (!canvas) return;

        const rect = canvas.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        setMousePos({ x: e.clientX, y: e.clientY });

        // Check NPC hover
        for (const npc of npcs) {
            const dist = Math.sqrt((x - npc.x) ** 2 + (y - npc.y) ** 2);
            if (dist < CONFIG.NPC_RADIUS) {
                setHoveredEntity({ type: 'npc', data: npc, position: { x: npc.x, y: npc.y } });
                return;
            }
        }

        // Check gate hover
        for (const gate of Object.values(gates)) {
            const dist = Math.sqrt((x - gate.position[0]) ** 2 + (y - gate.position[1]) ** 2);
            if (dist < CONFIG.GATE_RADIUS) {
                setHoveredEntity({ type: 'gate', data: gate, position: { x: gate.position[0], y: gate.position[1] } });
                return;
            }
        }

        setHoveredEntity(null);
    }, [npcs, gates, setHoveredEntity, setMousePos]);

    // Game loop
    useEffect(() => {
        if (!running) return;

        const loop = () => {
            setTick(tick + 1);
            updateNPCs();
            updateParticles();
            render();
        };

        const animId = requestAnimationFrame(loop);
        return () => cancelAnimationFrame(animId);
    }, [running, tick, setTick, updateNPCs, updateParticles, render]);

    // Initial render
    useEffect(() => {
        render();
    }, [render]);

    return (
        <canvas
            ref={canvasRef}
            width={CONFIG.WORLD_WIDTH}
            height={CONFIG.WORLD_HEIGHT}
            onMouseMove={handleMouseMove}
            onMouseLeave={() => setHoveredEntity(null)}
            style={{
                borderRadius: '12px',
                boxShadow: '0 0 30px rgba(0, 0, 0, 0.5)',
                cursor: 'crosshair'
            }}
        />
    );
}
