import { useEffect, useRef, useCallback } from 'react';
import { useGameStore, CONFIG } from '../store/gameStore';

export function useWebSocket() {
    const ws = useRef<WebSocket | null>(null);
    const {
        setConnected,
        setProviders,
        setTeams,
        setZones,
        setGates,
        updateNPC,
        npcs,
        setCommentary,
        addFeedItem,
        updateTeamScore,
        addParticle
    } = useGameStore();

    // Handle NPC decision
    const handleDecision = useCallback((decision: any) => {
        const npcId = decision.npc_id || decision.npcId;
        const npcName = decision.npc || decision.name;

        // Find NPC by ID or name
        const targetNPC = npcs.find(n => n.id === npcId || n.name === npcName);
        if (!targetNPC) return;

        const action = decision.action;
        const target = decision.target;
        const reason = decision.reason || '';
        const message = decision.message || '';

        switch (action) {
            case 'move':
            case 'explore':
                if (Array.isArray(target) && target.length >= 2) {
                    const [x, y] = target;
                    // Clamp to world bounds
                    const clampedX = Math.max(CONFIG.NPC_RADIUS, Math.min(CONFIG.WORLD_WIDTH - CONFIG.NPC_RADIUS, x));
                    const clampedY = Math.max(CONFIG.NPC_RADIUS, Math.min(CONFIG.WORLD_HEIGHT - CONFIG.NPC_RADIUS, y));

                    updateNPC(targetNPC.id, {
                        targetX: clampedX,
                        targetY: clampedY,
                        state: 'moving',
                        thought: reason
                    });

                    // Add movement particles
                    addParticle({
                        x: targetNPC.x,
                        y: targetNPC.y,
                        vx: (Math.random() - 0.5) * 0.5,
                        vy: (Math.random() - 0.5) * 0.5,
                        life: 30,
                        maxLife: 30,
                        color: targetNPC.team === 'red' ? '#ef4444' : '#3b82f6',
                        size: 3,
                        type: 'trail'
                    });
                }
                break;

            case 'talk':
            case 'taunt':
                if (message) {
                    addFeedItem({
                        npc: {
                            name: targetNPC.name,
                            team: targetNPC.team,
                            color: targetNPC.team === 'red' ? '#ef4444' : '#3b82f6'
                        },
                        text: message,
                        action: action
                    });
                    updateNPC(targetNPC.id, { thought: message });
                }
                break;

            case 'challenge':
                updateNPC(targetNPC.id, { state: 'challenging', thought: `Attempting ${target}...` });
                // Add thinking sparkles
                for (let i = 0; i < 5; i++) {
                    addParticle({
                        x: targetNPC.x + (Math.random() - 0.5) * 40,
                        y: targetNPC.y - 30 + (Math.random() - 0.5) * 20,
                        vx: (Math.random() - 0.5) * 1,
                        vy: -1 - Math.random(),
                        life: 40,
                        maxLife: 40,
                        color: '#a855f7',
                        size: 3,
                        type: 'thinking'
                    });
                }
                break;

            case 'idle':
            case 'wait':
                updateNPC(targetNPC.id, { state: 'idle', thought: reason || 'Observing...' });
                break;
        }
    }, [npcs, updateNPC, addFeedItem, addParticle]);

    // Connect to WebSocket
    const connect = useCallback(() => {
        if (ws.current?.readyState === WebSocket.OPEN) return;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        // Connect to Go backend on port 8080
        const host = window.location.hostname;
        ws.current = new WebSocket(`${protocol}//${host}:8080/ws`);

        ws.current.onopen = () => {
            console.log('🔌 WebSocket connected');
            setConnected(true);
        };

        ws.current.onclose = () => {
            console.log('🔌 WebSocket disconnected');
            setConnected(false);
            // Reconnect after 3 seconds
            setTimeout(connect, 3000);
        };

        ws.current.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        ws.current.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);

                switch (data.type) {
                    case 'init':
                        setProviders(data.slm || '--', data.brain || '--');
                        if (data.teams) setTeams(data.teams);
                        if (data.zones) setZones(data.zones);
                        if (data.gates) setGates(data.gates);
                        break;

                    case 'decision':
                        handleDecision(data);
                        break;

                    case 'batch_decisions':
                        if (data.decisions && Array.isArray(data.decisions)) {
                            const cacheHits = data.from_cache?.filter(Boolean).length || 0;
                            console.log(`📦 Batch: ${data.decisions.length} decisions (${cacheHits} cached)`);
                            data.decisions.forEach((dec: any) => {
                                if (dec) handleDecision(dec);
                            });
                        }
                        break;

                    case 'brain_strategy':
                    case 'commentary':
                        setCommentary(data.strategy || data.commentary || '');
                        break;

                    case 'challenge_result':
                        if (data.success) {
                            // Gate unlock particles
                            const gate = useGameStore.getState().gates[data.gateId];
                            if (gate) {
                                for (let i = 0; i < 30; i++) {
                                    const angle = (i / 30) * Math.PI * 2;
                                    addParticle({
                                        x: gate.position[0],
                                        y: gate.position[1],
                                        vx: Math.cos(angle) * (2 + Math.random() * 2),
                                        vy: Math.sin(angle) * (2 + Math.random() * 2),
                                        life: 60,
                                        maxLife: 60,
                                        color: ['#fbbf24', '#22c55e', '#a855f7'][i % 3],
                                        size: 5,
                                        type: 'unlock'
                                    });
                                }
                            }
                            // Update score
                            if (data.team && data.score) {
                                updateTeamScore(data.team, data.score);
                            }
                        }
                        break;

                    case 'game_state':
                        if (data.state?.teams) setTeams(data.state.teams);
                        break;
                }
            } catch (e) {
                console.error('Failed to parse WebSocket message:', e);
            }
        };
    }, [setConnected, setProviders, setTeams, setZones, setGates, handleDecision, setCommentary, addParticle, updateTeamScore]);

    // Send batch decision request
    const requestBatchDecisions = useCallback(() => {
        if (!ws.current || ws.current.readyState !== WebSocket.OPEN) return;

        const observations = npcs.map(npc => ({
            npc_id: npc.id,
            name: npc.name,
            team: npc.team,
            pos: [npc.x, npc.y],
            hp: npc.hp,
            energy: npc.energy,
            state: npc.state,
            memory_code: npc.memoryCode,
            nearby_npcs: npcs
                .filter(other => other.id !== npc.id)
                .map(other => {
                    const dx = other.x - npc.x;
                    const dy = other.y - npc.y;
                    const distance = Math.sqrt(dx * dx + dy * dy);
                    return {
                        id: other.id,
                        name: other.name,
                        team: other.team,
                        distance,
                        direction: Math.atan2(dy, dx),
                        state: other.state,
                        isTeammate: other.team === npc.team
                    };
                })
                .filter(n => n.distance < CONFIG.VISION_RANGE),
            nearby_gates: Object.values(useGameStore.getState().gates)
                .map(gate => {
                    const dx = gate.position[0] - npc.x;
                    const dy = gate.position[1] - npc.y;
                    return {
                        id: gate.id,
                        distance: Math.sqrt(dx * dx + dy * dy),
                        direction: Math.atan2(dy, dx),
                        unlocked: gate.unlocked,
                        requiresTeamwork: gate.requiresTeamwork
                    };
                })
                .filter(g => g.distance < CONFIG.VISION_RANGE)
        }));

        ws.current.send(JSON.stringify({
            type: 'batch_decisions',
            observations
        }));

        console.log(`📦 Batch request: ${observations.length} NPCs`);
    }, [npcs]);

    // Request commentary
    const requestCommentary = useCallback(() => {
        if (!ws.current || ws.current.readyState !== WebSocket.OPEN) return;

        const teams = useGameStore.getState().teams;
        ws.current.send(JSON.stringify({
            type: 'brain_request',
            summary: `Team Red: ${teams.red?.score || 0} pts, Team Blue: ${teams.blue?.score || 0} pts`
        }));
    }, []);

    useEffect(() => {
        connect();
        return () => {
            ws.current?.close();
        };
    }, [connect]);

    return { requestBatchDecisions, requestCommentary };
}
