/**
 * NPC Arena v2 - Game Engine
 * Browser-based AI arena with team competition
 */

// ============ CONFIGURATION ============
const CONFIG = {
    WORLD_WIDTH: 1200,
    WORLD_HEIGHT: 800,
    TICK_RATE: 60,
    DECISION_RATE: 0.25,  // 1 decision per 4 seconds per NPC
    NPC_RADIUS: 22,
    NPC_SPEED: 2.5,
    VISION_RANGE: 500,  // Increased so NPCs can see gates across zones
    INTERACTION_RANGE: 60,
    GATE_RADIUS: 30
};

// Team colors
const TEAM_COLORS = {
    red: { primary: '#ef4444', bg: 'rgba(239, 68, 68, 0.15)', glow: 'rgba(239, 68, 68, 0.4)' },
    blue: { primary: '#3b82f6', bg: 'rgba(59, 130, 246, 0.15)', glow: 'rgba(59, 130, 246, 0.4)' }
};

// NPC assignments
const NPC_TEAM_MAP = {
    'Explorer': 'red',
    'Scout': 'red',
    'Wanderer': 'blue',
    'Seeker': 'blue'
};

// Zone colors
const ZONE_THEMES = {
    neutral: { color: '#4a4a5a', name: 'Starting Grounds' },
    crystal: { color: '#06b6d4', name: 'Crystal Caverns' },
    forest: { color: '#22c55e', name: 'Whispering Woods' },
    void: { color: '#a855f7', name: 'The Nexus' }
};

// ============ GAME STATE ============
const gameState = {
    running: false,
    tick: 0,
    npcs: [],
    zones: {},
    gates: {},
    teams: {},
    chatBubbles: [],
    feedItems: [],
    activeChallenge: null,
    ws: null,
    lastDecisionTime: {}
};

// ============ NPC CLASS ============
class NPC {
    constructor(id, name, x, y) {
        this.id = id;
        this.name = name;
        this.x = x;
        this.y = y;
        this.targetX = x;
        this.targetY = y;
        this.angle = Math.random() * Math.PI * 2;
        this.hp = 100;
        this.energy = 100;
        this.state = 'idle';
        this.action = null;
        this.thought = '';
        this.team = NPC_TEAM_MAP[name] || 'red';
        this.trail = [];
        this.memoryCode = '';
    }

    get color() {
        return TEAM_COLORS[this.team].primary;
    }

    update() {
        // Move towards target
        const dx = this.targetX - this.x;
        const dy = this.targetY - this.y;
        const dist = Math.sqrt(dx * dx + dy * dy);

        if (dist > 5) {
            this.angle = Math.atan2(dy, dx);
            this.x += (dx / dist) * CONFIG.NPC_SPEED;
            this.y += (dy / dist) * CONFIG.NPC_SPEED;
            this.state = 'moving';

            // Add to trail
            this.trail.push({ x: this.x, y: this.y, age: 0 });
            if (this.trail.length > 25) this.trail.shift();
        } else {
            this.state = 'idle';
        }

        // Age trail
        this.trail = this.trail.map(p => ({ ...p, age: p.age + 1 }))
            .filter(p => p.age < 50);

        // Energy regen
        if (this.state === 'idle') {
            this.energy = Math.min(100, this.energy + 0.15);
        }

        // Clamp position
        this.x = Math.max(CONFIG.NPC_RADIUS, Math.min(CONFIG.WORLD_WIDTH - CONFIG.NPC_RADIUS, this.x));
        this.y = Math.max(CONFIG.NPC_RADIUS, Math.min(CONFIG.WORLD_HEIGHT - CONFIG.NPC_RADIUS, this.y));
    }

    setTarget(x, y) {
        this.targetX = Math.max(CONFIG.NPC_RADIUS, Math.min(CONFIG.WORLD_WIDTH - CONFIG.NPC_RADIUS, x));
        this.targetY = Math.max(CONFIG.NPC_RADIUS, Math.min(CONFIG.WORLD_HEIGHT - CONFIG.NPC_RADIUS, y));
    }

    getObservation(allNpcs, gates) {
        const nearbyNpcs = allNpcs
            .filter(n => n.id !== this.id)
            .map(n => ({
                id: n.id,
                name: n.name,
                team: n.team,
                distance: Math.sqrt((n.x - this.x) ** 2 + (n.y - this.y) ** 2),
                direction: Math.atan2(n.y - this.y, n.x - this.x),
                state: n.state,
                isTeammate: n.team === this.team
            }))
            .filter(n => n.distance < CONFIG.VISION_RANGE);

        const nearbyGates = Object.values(gates)
            .map(g => ({
                id: g.id,
                distance: Math.sqrt((g.position[0] - this.x) ** 2 + (g.position[1] - this.y) ** 2),
                unlocked: g.unlocked,
                requiresTeamwork: g.requiresTeamwork
            }))
            .filter(g => g.distance < CONFIG.VISION_RANGE);

        return {
            npc_id: this.id,
            name: this.name,
            team: this.team,
            pos: [Math.round(this.x), Math.round(this.y)],
            hp: this.hp,
            energy: Math.round(this.energy),
            state: this.state,
            nearby_npcs: nearbyNpcs,
            nearby_gates: nearbyGates,
            memory_code: this.memoryCode,
            current_action: this.action
        };
    }
}

// ============ CHAT BUBBLE ============
class ChatBubble {
    constructor(npc, message, type = 'thought') {
        this.npc = npc;
        this.message = message;
        this.type = type;
        this.opacity = 1;
        this.lifetime = 180; // frames
        this.age = 0;
    }

    update() {
        this.age++;
        if (this.age > this.lifetime * 0.7) {
            this.opacity = 1 - (this.age - this.lifetime * 0.7) / (this.lifetime * 0.3);
        }
    }

    isExpired() {
        return this.age >= this.lifetime;
    }
}

// ============ INITIALIZATION ============
function initGame() {
    // Create NPCs
    const npcData = [
        { name: 'Explorer', x: 150, y: 150 },
        { name: 'Scout', x: 250, y: 150 },
        { name: 'Wanderer', x: CONFIG.WORLD_WIDTH - 150, y: CONFIG.WORLD_HEIGHT - 150 },
        { name: 'Seeker', x: CONFIG.WORLD_WIDTH - 250, y: CONFIG.WORLD_HEIGHT - 150 }
    ];

    // Memory codes for each NPC
    const memoryCodes = ['A749', 'B312', 'C856', 'D427'];

    gameState.npcs = npcData.map((data, i) => {
        const npc = new NPC(`npc_${i}`, data.name, data.x, data.y);
        npc.memoryCode = memoryCodes[i];
        return npc;
    });

    // Create zones
    const halfW = CONFIG.WORLD_WIDTH / 2;
    const halfH = CONFIG.WORLD_HEIGHT / 2;

    gameState.zones = {
        start: { id: 'start', name: 'Starting Grounds', theme: 'neutral', bounds: { x: 0, y: 0, w: halfW, h: halfH }, unlocked: true },
        zone_2: { id: 'zone_2', name: 'Crystal Caverns', theme: 'crystal', bounds: { x: halfW, y: 0, w: halfW, h: halfH }, unlocked: false },
        zone_3: { id: 'zone_3', name: 'Whispering Woods', theme: 'forest', bounds: { x: 0, y: halfH, w: halfW, h: halfH }, unlocked: false },
        zone_4: { id: 'zone_4', name: 'The Nexus', theme: 'void', bounds: { x: halfW, y: halfH, w: halfW, h: halfH }, unlocked: false }
    };

    // Create gates
    gameState.gates = {
        gate_1_2: { id: 'gate_1_2', position: [halfW, halfH / 2], unlocked: false, requiresTeamwork: false, toZone: 'zone_2' },
        gate_1_3: { id: 'gate_1_3', position: [halfW / 2, halfH], unlocked: false, requiresTeamwork: true, toZone: 'zone_3' },
        gate_2_4: { id: 'gate_2_4', position: [halfW + halfW / 2, halfH], unlocked: false, requiresTeamwork: false, toZone: 'zone_4' },
        gate_3_4: { id: 'gate_3_4', position: [halfW, halfH + halfH / 2], unlocked: false, requiresTeamwork: true, toZone: 'zone_4' }
    };

    // Initialize teams
    gameState.teams = {
        red: { id: 'red', name: 'Team Red', score: 0, tokens: 50, members: ['Explorer', 'Scout'] },
        blue: { id: 'blue', name: 'Team Blue', score: 0, tokens: 50, members: ['Wanderer', 'Seeker'] }
    };

    updateUI();
    connectWebSocket();
}

// ============ RENDERING ============
function render() {
    const canvas = document.getElementById('game-canvas');
    const ctx = canvas.getContext('2d');

    // Scale canvas to container
    const container = canvas.parentElement;
    canvas.width = container.clientWidth;
    canvas.height = container.clientHeight;

    const scaleX = canvas.width / CONFIG.WORLD_WIDTH;
    const scaleY = canvas.height / CONFIG.WORLD_HEIGHT;

    ctx.save();
    ctx.scale(scaleX, scaleY);

    // Clear with gradient background
    const bgGrad = ctx.createLinearGradient(0, 0, CONFIG.WORLD_WIDTH, CONFIG.WORLD_HEIGHT);
    bgGrad.addColorStop(0, '#0a0a12');
    bgGrad.addColorStop(1, '#12121a');
    ctx.fillStyle = bgGrad;
    ctx.fillRect(0, 0, CONFIG.WORLD_WIDTH, CONFIG.WORLD_HEIGHT);

    // Draw zones
    drawZones(ctx);

    // Draw gates
    drawGates(ctx);

    // Draw grid (subtle)
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.03)';
    ctx.lineWidth = 1;
    for (let x = 0; x < CONFIG.WORLD_WIDTH; x += 50) {
        ctx.beginPath();
        ctx.moveTo(x, 0);
        ctx.lineTo(x, CONFIG.WORLD_HEIGHT);
        ctx.stroke();
    }
    for (let y = 0; y < CONFIG.WORLD_HEIGHT; y += 50) {
        ctx.beginPath();
        ctx.moveTo(0, y);
        ctx.lineTo(CONFIG.WORLD_WIDTH, y);
        ctx.stroke();
    }

    // Draw NPC trails
    gameState.npcs.forEach(npc => {
        if (npc.trail.length > 1) {
            ctx.beginPath();
            ctx.moveTo(npc.trail[0].x, npc.trail[0].y);
            for (let i = 1; i < npc.trail.length; i++) {
                ctx.lineTo(npc.trail[i].x, npc.trail[i].y);
            }
            ctx.strokeStyle = npc.color + '30';
            ctx.lineWidth = 4;
            ctx.lineCap = 'round';
            ctx.stroke();
        }
    });

    // Draw NPCs
    gameState.npcs.forEach(npc => drawNPC(ctx, npc));

    // Draw chat bubbles
    gameState.chatBubbles.forEach(bubble => drawChatBubble(ctx, bubble));

    ctx.restore();

    // Update tick counter
    document.getElementById('tick-counter').textContent = `Tick: ${gameState.tick}`;
}

function drawZones(ctx) {
    Object.values(gameState.zones).forEach(zone => {
        const b = zone.bounds;
        const theme = ZONE_THEMES[zone.theme] || ZONE_THEMES.neutral;

        // Zone background
        ctx.fillStyle = zone.unlocked ? theme.color + '15' : 'rgba(0,0,0,0.3)';
        ctx.fillRect(b.x, b.y, b.w, b.h);

        // Zone border
        ctx.strokeStyle = zone.unlocked ? theme.color + '60' : 'rgba(255,255,255,0.1)';
        ctx.lineWidth = 2;
        ctx.strokeRect(b.x + 1, b.y + 1, b.w - 2, b.h - 2);

        // Zone name
        ctx.font = '14px Inter, sans-serif';
        ctx.fillStyle = zone.unlocked ? theme.color : 'rgba(255,255,255,0.3)';
        ctx.textAlign = 'center';
        ctx.fillText(zone.name, b.x + b.w / 2, b.y + 25);

        // Lock icon if not unlocked
        if (!zone.unlocked && zone.id !== 'start') {
            ctx.font = '20px Arial';
            ctx.fillText('🔒', b.x + b.w / 2, b.y + b.h / 2);
        }
    });
}

function drawGates(ctx) {
    Object.values(gameState.gates).forEach(gate => {
        const [x, y] = gate.position;

        // Gate glow
        if (!gate.unlocked) {
            const glow = ctx.createRadialGradient(x, y, 0, x, y, CONFIG.GATE_RADIUS * 1.5);
            glow.addColorStop(0, gate.requiresTeamwork ? 'rgba(168, 85, 247, 0.3)' : 'rgba(251, 191, 36, 0.3)');
            glow.addColorStop(1, 'transparent');
            ctx.fillStyle = glow;
            ctx.beginPath();
            ctx.arc(x, y, CONFIG.GATE_RADIUS * 1.5, 0, Math.PI * 2);
            ctx.fill();
        }

        // Gate icon
        ctx.font = '24px Arial';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(gate.unlocked ? '🚪' : (gate.requiresTeamwork ? '🔐' : '🔒'), x, y);

        // Teamwork indicator
        if (gate.requiresTeamwork && !gate.unlocked) {
            ctx.font = '10px Inter, sans-serif';
            ctx.fillStyle = '#a855f7';
            ctx.fillText('2 players', x, y + 25);
        }
    });
}

function drawNPC(ctx, npc) {
    // Vision range (very subtle)
    ctx.beginPath();
    ctx.arc(npc.x, npc.y, CONFIG.VISION_RANGE, 0, Math.PI * 2);
    ctx.fillStyle = npc.color + '05';
    ctx.fill();

    // NPC body with glow
    ctx.shadowColor = npc.color;
    ctx.shadowBlur = npc.state === 'idle' ? 8 : 15;

    ctx.beginPath();
    ctx.arc(npc.x, npc.y, CONFIG.NPC_RADIUS, 0, Math.PI * 2);

    // Gradient fill
    const grad = ctx.createRadialGradient(npc.x - 5, npc.y - 5, 0, npc.x, npc.y, CONFIG.NPC_RADIUS);
    grad.addColorStop(0, npc.color);
    grad.addColorStop(1, npc.color + '80');
    ctx.fillStyle = grad;
    ctx.fill();

    // Border
    ctx.strokeStyle = '#fff';
    ctx.lineWidth = 2;
    ctx.stroke();

    ctx.shadowBlur = 0;

    // Direction indicator
    const dirX = npc.x + Math.cos(npc.angle) * CONFIG.NPC_RADIUS;
    const dirY = npc.y + Math.sin(npc.angle) * CONFIG.NPC_RADIUS;
    ctx.beginPath();
    ctx.moveTo(npc.x, npc.y);
    ctx.lineTo(dirX, dirY);
    ctx.strokeStyle = '#fff';
    ctx.lineWidth = 3;
    ctx.lineCap = 'round';
    ctx.stroke();

    // Name label
    ctx.font = 'bold 11px Inter, sans-serif';
    ctx.textAlign = 'center';
    ctx.fillStyle = '#fff';
    ctx.fillText(npc.name, npc.x, npc.y - CONFIG.NPC_RADIUS - 10);

    // Team indicator
    ctx.font = '8px Inter, sans-serif';
    ctx.fillStyle = npc.color;
    ctx.fillText(npc.team.toUpperCase(), npc.x, npc.y - CONFIG.NPC_RADIUS - 22);
}

function drawChatBubble(ctx, bubble) {
    if (bubble.opacity <= 0) return;

    const npc = bubble.npc;
    const x = npc.x;
    const y = npc.y - CONFIG.NPC_RADIUS - 50;

    const maxWidth = 120;
    ctx.font = '10px Inter, sans-serif';
    const text = bubble.message.length > 30 ? bubble.message.substring(0, 30) + '...' : bubble.message;
    const metrics = ctx.measureText(text);
    const width = Math.min(metrics.width + 12, maxWidth);
    const height = 22;

    ctx.globalAlpha = bubble.opacity;

    // Bubble background
    ctx.fillStyle = '#1a1a25';
    ctx.strokeStyle = npc.color + '80';
    ctx.lineWidth = 1;

    // Rounded rect
    const r = 6;
    ctx.beginPath();
    ctx.moveTo(x - width / 2 + r, y - height / 2);
    ctx.lineTo(x + width / 2 - r, y - height / 2);
    ctx.quadraticCurveTo(x + width / 2, y - height / 2, x + width / 2, y - height / 2 + r);
    ctx.lineTo(x + width / 2, y + height / 2 - r);
    ctx.quadraticCurveTo(x + width / 2, y + height / 2, x + width / 2 - r, y + height / 2);
    ctx.lineTo(x - width / 2 + r, y + height / 2);
    ctx.quadraticCurveTo(x - width / 2, y + height / 2, x - width / 2, y + height / 2 - r);
    ctx.lineTo(x - width / 2, y - height / 2 + r);
    ctx.quadraticCurveTo(x - width / 2, y - height / 2, x - width / 2 + r, y - height / 2);
    ctx.closePath();
    ctx.fill();
    ctx.stroke();

    // Text
    ctx.fillStyle = '#e2e8f0';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, x, y);

    ctx.globalAlpha = 1;
}

// ============ GAME LOOP ============
function gameLoop() {
    if (!gameState.running) return;

    gameState.tick++;

    // Update NPCs
    gameState.npcs.forEach(npc => {
        npc.update();

        // Check for gate interactions
        Object.values(gameState.gates).forEach(gate => {
            if (gate.unlocked) return;
            const dist = Math.sqrt((gate.position[0] - npc.x) ** 2 + (gate.position[1] - npc.y) ** 2);
            if (dist < CONFIG.INTERACTION_RANGE) {
                // Could trigger challenge here
            }
        });
    });

    // Update chat bubbles
    gameState.chatBubbles.forEach(b => b.update());
    gameState.chatBubbles = gameState.chatBubbles.filter(b => !b.isExpired());

    // Request AI decisions periodically
    const now = Date.now();
    gameState.npcs.forEach(npc => {
        const lastDecision = gameState.lastDecisionTime[npc.id] || 0;
        if (now - lastDecision > 1000 / CONFIG.DECISION_RATE) {
            requestDecision(npc);
            gameState.lastDecisionTime[npc.id] = now;
        }
    });

    // Request commentary every 10 seconds
    if (!gameState.lastCommentaryTime) gameState.lastCommentaryTime = now;
    if (now - gameState.lastCommentaryTime > 10000) {
        requestCommentary();
        gameState.lastCommentaryTime = now;
    }

    // Check for zone generation every 30 seconds
    if (!gameState.lastZoneCheckTime) gameState.lastZoneCheckTime = now;
    if (now - gameState.lastZoneCheckTime > 30000) {
        checkZoneGeneration();
        gameState.lastZoneCheckTime = now;
    }

    // Render
    render();
    updateUI();

    requestAnimationFrame(gameLoop);
}

// ============ AI INTEGRATION ============
function requestDecision(npc) {
    const observation = npc.getObservation(gameState.npcs, gameState.gates);

    if (gameState.ws && gameState.ws.readyState === WebSocket.OPEN) {
        gameState.ws.send(JSON.stringify({
            type: 'decision_request',
            observation: observation
        }));
    } else {
        simulateAIDecision(npc);
    }
}

function simulateAIDecision(npc) {
    // Demo mode exploration
    const thoughts = [
        "Exploring the area...",
        "Looking for the gate...",
        "Where's my teammate?",
        "Moving strategically",
        "Checking surroundings"
    ];

    // Find nearest unlocked gate or random exploration
    const lockedGates = Object.values(gameState.gates)
        .filter(g => !g.unlocked)
        .map(g => ({
            ...g,
            dist: Math.sqrt((g.position[0] - npc.x) ** 2 + (g.position[1] - npc.y) ** 2)
        }))
        .sort((a, b) => a.dist - b.dist);

    if (lockedGates.length > 0 && Math.random() > 0.3) {
        const target = lockedGates[0];
        npc.setTarget(target.position[0], target.position[1]);
        npc.thought = `Heading to gate...`;
    } else {
        const angle = Math.random() * Math.PI * 2;
        const dist = 50 + Math.random() * 150;
        npc.setTarget(
            npc.x + Math.cos(angle) * dist,
            npc.y + Math.sin(angle) * dist
        );
        npc.thought = thoughts[Math.floor(Math.random() * thoughts.length)];
    }

    // Add chat bubble
    if (Math.random() > 0.7) {
        addChatBubble(npc, npc.thought);
    }

    addFeedItem(npc, npc.thought, 'explore');
}

function handleAIResponse(data) {
    const npc = gameState.npcs.find(n => n.id === data.npc_id);
    if (!npc) return;

    npc.thought = data.reason || '';
    npc.action = data;

    switch (data.action) {
        case 'move':
            // LLM returns "target" not "target_pos"
            const target = data.target || data.target_pos;
            if (target && Array.isArray(target) &&
                typeof target[0] === 'number' && typeof target[1] === 'number') {
                npc.setTarget(target[0], target[1]);
                console.log(`${npc.name} moving to (${target[0]}, ${target[1]})`);
            } else {
                // Invalid target, explore randomly
                const newX = npc.x + (Math.random() - 0.5) * 100;
                const newY = npc.y + (Math.random() - 0.5) * 100;
                npc.setTarget(
                    Math.max(50, Math.min(CONFIG.WORLD_WIDTH - 50, newX)),
                    Math.max(50, Math.min(CONFIG.WORLD_HEIGHT - 50, newY))
                );
                console.log(`${npc.name} got invalid target, exploring`);
            }
            break;
        case 'challenge':
            // Handle challenge attempt with debouncing
            const gateId = data.target;
            const gate = gameState.gates[gateId];

            // Skip if gate already unlocked OR NPC already challenging OR recently challenged
            if (!gate || gate.unlocked) {
                console.log(`${npc.name}: Gate ${gateId} already unlocked, moving away`);
                moveNpcAwayFromGate(npc, gate);
                break;
            }

            // Check if this NPC recently attempted this gate (debounce)
            const challengeKey = `${npc.name}_${gateId}`;
            const now = Date.now();
            if (gameState.challengeCooldowns && gameState.challengeCooldowns[challengeKey]) {
                if (now - gameState.challengeCooldowns[challengeKey] < 10000) { // 10 sec cooldown
                    console.log(`${npc.name}: Waiting for challenge result on ${gateId}`);
                    break;
                }
            }

            // Initialize cooldowns if needed
            if (!gameState.challengeCooldowns) gameState.challengeCooldowns = {};
            gameState.challengeCooldowns[challengeKey] = now;

            // Mark NPC as challenging
            npc.state = 'challenging';

            // Show challenge and solve it
            console.log(`${npc.name} attempting challenge at ${gateId}`);
            handleNpcChallenge(npc, gateId, gate);
            break;
        case 'signal':
            // Send message to teammate
            addFeedItem(npc, 'Signaling teammate...', 'signal');
            break;
        case 'wait':
            // Waiting for teammate
            addFeedItem(npc, 'Waiting for teammate...', 'wait');
            break;
        case 'talk':
            // NPC is talking to another NPC
            handleNpcDialogue(npc, data.target, data.message, 'talk');
            break;
        case 'taunt':
            // NPC is taunting an opponent
            handleNpcDialogue(npc, data.target, data.message, 'taunt');
            break;
        default:
            // Unknown action - move randomly
            const randX = npc.x + (Math.random() - 0.5) * 80;
            const randY = npc.y + (Math.random() - 0.5) * 80;
            npc.setTarget(
                Math.max(50, Math.min(CONFIG.WORLD_WIDTH - 50, randX)),
                Math.max(50, Math.min(CONFIG.WORLD_HEIGHT - 50, randY))
            );
            break;
    }

    if (npc.thought) {
        addChatBubble(npc, npc.thought);
        addFeedItem(npc, npc.thought, data.action);
    }
}

// ============ WEBSOCKET ============
function connectWebSocket() {
    try {
        gameState.ws = new WebSocket('ws://localhost:8080/ws');

        gameState.ws.onopen = () => {
            document.getElementById('connection-status').textContent = '● Online';
            document.getElementById('connection-status').className = 'status-badge connected';
        };

        gameState.ws.onmessage = (event) => {
            const data = JSON.parse(event.data);

            switch (data.type) {
                case 'init':
                    // Initial state from server
                    document.getElementById('slm-provider').textContent = data.slm || '--';
                    document.getElementById('brain-provider').textContent = data.brain || '--';
                    if (data.teams) {
                        gameState.teams = data.teams;
                    }
                    if (data.zones) {
                        Object.assign(gameState.zones, data.zones);
                    }
                    if (data.gates) {
                        Object.assign(gameState.gates, data.gates);
                    }
                    updateUI();
                    break;

                case 'decision':
                    handleAIResponse(data);
                    break;

                case 'brain_strategy':
                    updateCommentary(data.strategy);
                    break;

                case 'challenge_active':
                    showChallengeModal(data.challenge);
                    break;

                case 'challenge_result':
                    handleChallengeResult(data);
                    break;

                case 'message_sent':
                    // Show team message as chat bubble
                    const from = gameState.npcs.find(n => n.name === data.from);
                    if (from) {
                        addChatBubble(from, `→ ${data.to}: ${data.message}`);
                    }
                    break;

                case 'game_state':
                    // Full state update
                    if (data.state.teams) {
                        gameState.teams = data.state.teams;
                    }
                    updateUI();
                    break;

                case 'commentary':
                    // Live commentary from LLM
                    updateCommentary(data.commentary);
                    break;

                case 'zone_generated':
                    // New zone was generated by Gemini
                    if (data.zones) {
                        gameState.zones = data.zones;
                    }
                    if (data.gates) {
                        gameState.gates = data.gates;
                    }
                    addFeedItem(
                        { name: 'World', team: 'red', color: '#a855f7' },
                        `🌍 New zone: ${data.zone.name}!`,
                        'zone'
                    );
                    render();
                    break;
            }
        };

        gameState.ws.onclose = () => {
            document.getElementById('connection-status').textContent = '● Offline';
            document.getElementById('connection-status').className = 'status-badge disconnected';
            setTimeout(connectWebSocket, 3000);
        };

        gameState.ws.onerror = () => {
            console.log('WebSocket error - running in demo mode');
        };
    } catch (e) {
        console.log('WebSocket not available - running in demo mode');
    }
}

// ============ UI UPDATES ============
function updateUI() {
    updateTeamList();
    updateScores();
}

function updateTeamList() {
    const container = document.getElementById('team-list');
    container.innerHTML = Object.values(gameState.teams).map(team => `
        <div class="team-card ${team.id}">
            <div class="team-info">
                <div class="team-label">${team.name}</div>
                <div class="team-members">${team.members?.join(', ') || ''}</div>
            </div>
            <div class="team-tokens">
                <span>🪙</span>
                <span>${team.tokens || 0}</span>
            </div>
        </div>
    `).join('');
}

function updateScores() {
    const red = gameState.teams.red || { score: 0 };
    const blue = gameState.teams.blue || { score: 0 };
    document.getElementById('score-red').textContent = red.score || 0;
    document.getElementById('score-blue').textContent = blue.score || 0;
}

function addChatBubble(npc, message) {
    const bubble = new ChatBubble(npc, message);
    gameState.chatBubbles.push(bubble);
}

function addFeedItem(npc, text, action) {
    const feed = document.getElementById('live-feed');
    const item = document.createElement('div');
    item.className = 'feed-item';
    item.innerHTML = `
        <div class="feed-avatar" style="background: ${TEAM_COLORS[npc.team].bg}; color: ${npc.color}">
            ${npc.name[0]}
        </div>
        <div class="feed-content">
            <div class="feed-name" style="color: ${npc.color}">${npc.name}</div>
            <div class="feed-text">${text}</div>
            <span class="feed-action">${action}</span>
        </div>
    `;

    feed.insertBefore(item, feed.firstChild);

    // Keep only last 15 items
    while (feed.children.length > 15) {
        feed.removeChild(feed.lastChild);
    }
}

function updateCommentary(text) {
    const container = document.getElementById('commentary');
    container.innerHTML = `"${text}"`;
}

function showChallengeModal(challenge) {
    const modal = document.getElementById('challenge-modal');
    const title = document.getElementById('challenge-title');
    const desc = document.getElementById('challenge-description');
    const participants = document.getElementById('challenge-participants');
    const thinking = document.getElementById('challenge-thinking');
    const thinkingContent = document.getElementById('thinking-content');
    const answer = document.getElementById('challenge-answer');
    const answerContent = document.getElementById('answer-content');
    const result = document.getElementById('challenge-result');
    const status = document.getElementById('challenge-status');

    // Reset all sections
    thinking.style.display = 'block';
    thinkingContent.innerHTML = '<div class="thinking-dots"><span></span><span></span><span></span></div>';
    answer.classList.add('hidden');
    result.classList.add('hidden');
    result.className = 'challenge-result hidden';

    // Set title and description
    title.textContent = `🧩 ${challenge.name || 'The Recall'}`;
    desc.textContent = challenge.prompt || 'Solve this challenge to unlock the gate!';

    // Show participants
    const participantList = challenge.participants || ['AI'];
    participants.innerHTML = participantList.map(p => {
        const npc = gameState.npcs.find(n => n.name === p);
        const team = npc ? npc.team : 'blue';
        return `<span class="participant-chip team-${team}">🤖 ${p}</span>`;
    }).join('');

    // Status
    status.textContent = challenge.requires_teamwork ? '👥 Requires both teammates!' : '🎯 Solo challenge';

    modal.classList.remove('hidden');
    gameState.activeChallenge = challenge;

    // Add close button handler
    document.getElementById('challenge-close').onclick = hideChallengeModal;

    // Simulate AI thinking process
    simulateAIThinking(challenge);
}

function simulateAIThinking(challenge) {
    const thinkingContent = document.getElementById('thinking-content');
    const answer = document.getElementById('challenge-answer');
    const answerContent = document.getElementById('answer-content');
    const result = document.getElementById('challenge-result');
    const thinking = document.getElementById('challenge-thinking');

    const thinkingSteps = [
        "Reading the challenge...",
        "Recalling my memory code...",
        `Thinking: The code was assigned earlier...`,
        "Formulating my answer..."
    ];

    let stepIndex = 0;

    // Show thinking steps with delay
    const thinkInterval = setInterval(() => {
        if (stepIndex < thinkingSteps.length) {
            thinkingContent.innerHTML = `<em>${thinkingSteps[stepIndex]}</em>`;
            stepIndex++;
        } else {
            clearInterval(thinkInterval);

            // Show answer after thinking
            setTimeout(() => {
                thinking.style.display = 'none';

                // Get the NPC's memory code or simulate
                const npcName = challenge.participants?.[0] || 'Explorer';
                const npc = gameState.npcs.find(n => n.name === npcName);
                const memoryCode = npc?.memoryCode || 'A749';

                answer.classList.remove('hidden');
                answerContent.innerHTML = `<strong>${memoryCode}</strong>`;

                // Simulate result after a brief moment
                setTimeout(() => {
                    showChallengeResult(true, "Correct! Memory recall successful.", 50);
                }, 1500);
            }, 500);
        }
    }, 800);
}

function showChallengeResult(success, feedback, points) {
    const result = document.getElementById('challenge-result');
    const resultIcon = document.getElementById('result-icon');
    const resultText = document.getElementById('result-text');
    const resultFeedback = document.getElementById('result-feedback');

    result.classList.remove('hidden', 'success', 'failure');
    result.classList.add(success ? 'success' : 'failure');

    resultIcon.textContent = success ? '✅' : '❌';
    resultText.textContent = success ? `+${points} Points!` : 'Challenge Failed';
    resultFeedback.textContent = feedback;

    // Mark the gate as unlocked if we have an active challenge
    if (success && gameState.activeChallenge?.gate_id) {
        const gate = gameState.gates[gameState.activeChallenge.gate_id];
        if (gate) {
            gate.unlocked = true;
            console.log(`Gate ${gameState.activeChallenge.gate_id} unlocked!`);

            // Move all NPCs away from this gate
            const gatePos = gate.position;
            gameState.npcs.forEach(npc => {
                const dist = Math.sqrt((npc.x - gatePos[0]) ** 2 + (npc.y - gatePos[1]) ** 2);
                if (dist < 100) {
                    moveNpcAwayFromGate(npc, gate);
                }
            });
        }
    }

    // Auto-close after showing result
    setTimeout(() => {
        hideChallengeModal();
        addFeedItem({ name: 'System', team: 'blue' },
            success ? `Gate unlocked! +${points} points` : 'Challenge failed',
            success ? 'success' : 'failure');
    }, 3000);
}

function hideChallengeModal() {
    document.getElementById('challenge-modal').classList.add('hidden');
    gameState.activeChallenge = null;
}

// Handle NPC challenge without modal (runs in background)
function handleNpcChallenge(npc, gateId, gate) {
    // Only one NPC per gate at a time
    if (gameState.gatesBeingChallenged && gameState.gatesBeingChallenged[gateId]) {
        console.log(`Gate ${gateId} already being challenged`);
        npc.state = 'idle';
        return;
    }

    // Mark gate as being challenged
    if (!gameState.gatesBeingChallenged) gameState.gatesBeingChallenged = {};
    gameState.gatesBeingChallenged[gateId] = true;

    addFeedItem(npc, `Solving challenge at ${gateId}...`, 'challenge');

    // Simulate thinking time (2-4 seconds)
    const thinkTime = 2000 + Math.random() * 2000;

    setTimeout(() => {
        // Challenge solved!
        gate.unlocked = true;
        delete gameState.gatesBeingChallenged[gateId];
        npc.state = 'idle';

        // Add points to team
        const team = npc.team;
        if (gameState.teams) {
            const teamData = gameState.teams.find(t => t.name.toLowerCase() === team);
            if (teamData) {
                teamData.score = (teamData.score || 0) + 50;
            }
        }

        // Show success notification
        addFeedItem(npc, `🎉 Unlocked ${gateId}! +50 points`, 'success');
        updateTeamScores();

        // Move NPC away from gate
        moveNpcAwayFromGate(npc, gate);

        console.log(`${npc.name} unlocked ${gateId}`);
    }, thinkTime);
}

// Move NPC away from a gate after unlocking
function moveNpcAwayFromGate(npc, gate) {
    if (!gate || !gate.position) return;

    // Move in random direction away from gate
    const angle = Math.random() * Math.PI * 2;
    const dist = 150 + Math.random() * 100;
    const newX = npc.x + Math.cos(angle) * dist;
    const newY = npc.y + Math.sin(angle) * dist;

    npc.setTarget(
        Math.max(50, Math.min(CONFIG.WORLD_WIDTH - 50, newX)),
        Math.max(50, Math.min(CONFIG.WORLD_HEIGHT - 50, newY))
    );
    npc.state = 'idle';
}

// Handle NPC dialogue (talk or taunt)
function handleNpcDialogue(speaker, targetName, message, type) {
    if (!message) message = type === 'taunt' ? "You're going down!" : "Hey there!";

    const targetNpc = gameState.npcs.find(n => n.name === targetName);
    const isOpponent = targetNpc && targetNpc.team !== speaker.team;

    // Show speech bubble on the speaker
    addChatBubble(speaker, message, type);

    // Create rich feed item
    const icon = type === 'taunt' ? '😤' : '💬';
    const relation = isOpponent ? '(opponent)' : '(teammate)';
    addFeedItem(speaker, `${icon} To ${targetName} ${relation}: "${message}"`, type);

    // If it's a taunt, increase tension and maybe trigger response
    if (type === 'taunt' && targetNpc) {
        // Increase tension meter
        const tensionFill = document.getElementById('tension-fill');
        if (tensionFill) {
            const current = parseInt(tensionFill.style.width) || 30;
            tensionFill.style.width = Math.min(100, current + 10) + '%';
        }

        // Maybe the target responds (50% chance, slight delay)
        if (Math.random() > 0.5) {
            setTimeout(() => {
                const responses = [
                    "Yeah right, watch me!",
                    "😂 In your dreams!",
                    "We'll see about that!",
                    "Bring it on!",
                    "Talk is cheap!"
                ];
                const response = responses[Math.floor(Math.random() * responses.length)];
                addChatBubble(targetNpc, response, 'taunt');
                addFeedItem(targetNpc, `😤 Response: "${response}"`, 'taunt');
            }, 1500);
        }
    }

    // If it's talk between teammates, show cooperation
    if (type === 'talk' && !isOpponent && targetNpc) {
        setTimeout(() => {
            const responses = [
                "Let's do this!",
                "I'm with you!",
                "Good thinking!",
                "On my way!"
            ];
            const response = responses[Math.floor(Math.random() * responses.length)];
            addChatBubble(targetNpc, response, 'talk');
            addFeedItem(targetNpc, `💬 Reply: "${response}"`, 'talk');
        }, 1200);
    }
}

// Update team scores in the UI
function updateTeamScores() {
    const teamList = document.getElementById('team-list');
    if (!teamList || !gameState.teams) return;

    // Update score displays in header
    const redScore = gameState.teams.find(t => t.name.toLowerCase() === 'red')?.score || 0;
    const blueScore = gameState.teams.find(t => t.name.toLowerCase() === 'blue')?.score || 0;

    const redScoreEl = document.getElementById('red-score');
    const blueScoreEl = document.getElementById('blue-score');
    if (redScoreEl) redScoreEl.textContent = redScore;
    if (blueScoreEl) blueScoreEl.textContent = blueScore;
}

function submitChallengeResponse(response) {
    if (gameState.ws && gameState.ws.readyState === WebSocket.OPEN && gameState.activeChallenge) {
        gameState.ws.send(JSON.stringify({
            type: 'challenge_response',
            gate_id: gameState.activeChallenge.gate_id,
            npc: 'Player', // For human players
            response: response
        }));
    }
}

function handleChallengeResult(data) {
    hideChallengeModal();

    // Update teams
    if (data.teams) {
        gameState.teams = data.teams;
    }

    // Unlock gate if success
    if (data.success && data.gate_id) {
        const gate = gameState.gates[data.gate_id];
        if (gate) {
            gate.unlocked = true;
            // Unlock destination zone
            const destZone = gameState.zones[gate.toZone];
            if (destZone) destZone.unlocked = true;
        }
    }

    // Add to feed
    addFeedItem(
        { name: 'System', team: 'red', color: data.success ? '#22c55e' : '#ef4444' },
        data.feedback,
        data.success ? 'success' : 'failed'
    );

    updateUI();
}

// ============ CONTROLS ============
document.getElementById('btn-start').addEventListener('click', () => {
    if (!gameState.running) {
        gameState.running = true;
        gameLoop();
    }
});

document.getElementById('btn-pause').addEventListener('click', () => {
    gameState.running = false;
});

document.getElementById('btn-reset').addEventListener('click', () => {
    gameState.running = false;
    gameState.tick = 0;
    gameState.chatBubbles = [];
    document.getElementById('live-feed').innerHTML = '';
    document.getElementById('commentary').innerHTML = '';
    initGame();
    render();
});

// ============ PHASE 3: COMMENTARY & ZONE GENERATION ============

function requestCommentary() {
    if (gameState.ws && gameState.ws.readyState === WebSocket.OPEN) {
        // Collect recent events for context
        const events = gameState.feedItems.slice(0, 3).map(item => ({
            event: item.action,
            description: `${item.name}: ${item.text}`
        }));

        gameState.ws.send(JSON.stringify({
            type: 'get_commentary',
            events: events
        }));
    }
}

function checkZoneGeneration() {
    if (gameState.ws && gameState.ws.readyState === WebSocket.OPEN) {
        gameState.ws.send(JSON.stringify({
            type: 'check_zone_generation'
        }));
    }
}

// ============ HUMAN PLAYER INTERFACE (Click-Based) ============

// Canvas click handler
document.getElementById('game-canvas').addEventListener('click', (e) => {
    if (!gameState.running) return;

    const canvas = e.target;
    const rect = canvas.getBoundingClientRect();
    const scaleX = CONFIG.WORLD_WIDTH / canvas.width;
    const scaleY = CONFIG.WORLD_HEIGHT / canvas.height;

    const clickX = (e.clientX - rect.left) * scaleX;
    const clickY = (e.clientY - rect.top) * scaleY;

    // Check if clicked on a gate
    const clickedGate = findGateAtPosition(clickX, clickY);
    if (clickedGate && !clickedGate.unlocked) {
        handleGateClick(clickedGate);
        return;
    }

    // Check if clicked on an NPC (for interaction options)
    const clickedNpc = findNpcAtPosition(clickX, clickY);
    if (clickedNpc) {
        handleNpcClick(clickedNpc);
        return;
    }

    // Otherwise, it's a move command - but this is AI-controlled for now
    // Human player would control their own NPC here
    console.log(`Clicked at (${Math.round(clickX)}, ${Math.round(clickY)})`);
});

function findGateAtPosition(x, y) {
    for (const gate of Object.values(gameState.gates)) {
        const dist = Math.sqrt(
            (gate.position[0] - x) ** 2 +
            (gate.position[1] - y) ** 2
        );
        if (dist < CONFIG.GATE_RADIUS * 1.5) {
            return gate;
        }
    }
    return null;
}

function findNpcAtPosition(x, y) {
    for (const npc of gameState.npcs) {
        const dist = Math.sqrt((npc.x - x) ** 2 + (npc.y - y) ** 2);
        if (dist < CONFIG.NPC_RADIUS * 1.5) {
            return npc;
        }
    }
    return null;
}

function handleGateClick(gate) {
    // Show challenge for this gate
    addFeedItem(
        { name: 'You', team: 'red', color: '#fbbf24' },
        `Clicked gate ${gate.id}`,
        'click'
    );

    // Request challenge start via WebSocket
    if (gameState.ws && gameState.ws.readyState === WebSocket.OPEN) {
        gameState.ws.send(JSON.stringify({
            type: 'challenge_start',
            gate_id: gate.id,
            npc: 'Human'  // Treating human as an NPC
        }));
    }
}

function handleNpcClick(npc) {
    // Show NPC info or interaction options
    addFeedItem(
        { name: 'Info', team: npc.team, color: npc.color },
        `${npc.name}: ${npc.state}, Energy: ${Math.round(npc.energy)}%`,
        'info'
    );

    // If it's a teammate, could show communication options
    // For now just show info
}

// Store feed items for commentary context
gameState.feedItems = [];
const originalAddFeedItem = addFeedItem;
addFeedItem = function (npc, text, action) {
    gameState.feedItems.unshift({ name: npc.name, text, action });
    if (gameState.feedItems.length > 20) {
        gameState.feedItems.pop();
    }
    originalAddFeedItem(npc, text, action);
};

// ============ START ============
window.addEventListener('load', () => {
    initGame();
    render();
});

window.addEventListener('resize', render);
