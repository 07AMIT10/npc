import { create } from 'zustand';

// Types
export interface Position {
    x: number;
    y: number;
}

export interface NPC {
    id: string;
    name: string;
    x: number;
    y: number;
    targetX: number;
    targetY: number;
    angle: number;
    hp: number;
    energy: number;
    state: 'idle' | 'moving' | 'challenging';
    action: any;
    thought: string;
    team: 'red' | 'blue';
    trail: { x: number; y: number; age: number }[];
    memoryCode: string;
}

export interface Zone {
    id: string;
    name: string;
    theme: string;
    bounds: { x: number; y: number; w: number; h: number };
    unlocked: boolean;
}

export interface Gate {
    id: string;
    position: [number, number];
    unlocked: boolean;
    requiresTeamwork: boolean;
    toZone: string;
}

export interface Team {
    id: string;
    name: string;
    score: number;
    tokens: number;
    members: string[];
}

export interface Particle {
    id: number;
    x: number;
    y: number;
    vx: number;
    vy: number;
    life: number;
    maxLife: number;
    color: string;
    size: number;
    type: 'trail' | 'unlock' | 'thinking' | 'score';
}

export interface FeedItem {
    id: number;
    npc: { name: string; team: string; color: string };
    text: string;
    action: string;
    timestamp: number;
}

export interface HoveredEntity {
    type: 'npc' | 'gate' | 'zone';
    data: any;
    position: Position;
}

interface GameState {
    // Game state
    running: boolean;
    tick: number;

    // Entities
    npcs: NPC[];
    zones: Record<string, Zone>;
    gates: Record<string, Gate>;
    teams: Record<string, Team>;

    // Visual effects
    particles: Particle[];
    feedItems: FeedItem[];
    commentary: string;

    // UI state
    hoveredEntity: HoveredEntity | null;
    mousePos: Position;

    // Connection
    connected: boolean;
    slmProvider: string;
    brainProvider: string;

    // Actions
    setRunning: (running: boolean) => void;
    setTick: (tick: number) => void;
    updateNPC: (id: string, updates: Partial<NPC>) => void;
    setNPCs: (npcs: NPC[]) => void;
    setZones: (zones: Record<string, Zone>) => void;
    setGates: (gates: Record<string, Gate>) => void;
    setTeams: (teams: Record<string, Team>) => void;
    updateTeamScore: (teamId: string, score: number) => void;
    addParticle: (particle: Omit<Particle, 'id'>) => void;
    updateParticles: () => void;
    addFeedItem: (item: Omit<FeedItem, 'id' | 'timestamp'>) => void;
    setCommentary: (text: string) => void;
    setHoveredEntity: (entity: HoveredEntity | null) => void;
    setMousePos: (pos: Position) => void;
    setConnected: (connected: boolean) => void;
    setProviders: (slm: string, brain: string) => void;
    reset: () => void;
}

// Team colors
export const TEAM_COLORS = {
    red: { primary: '#ef4444', bg: 'rgba(239, 68, 68, 0.15)', glow: 'rgba(239, 68, 68, 0.4)' },
    blue: { primary: '#3b82f6', bg: 'rgba(59, 130, 246, 0.15)', glow: 'rgba(59, 130, 246, 0.4)' }
};

// NPC to team mapping
export const NPC_TEAM_MAP: Record<string, 'red' | 'blue'> = {
    'Explorer': 'red',
    'Scout': 'red',
    'Wanderer': 'blue',
    'Seeker': 'blue'
};

// Config
export const CONFIG = {
    WORLD_WIDTH: 1200,
    WORLD_HEIGHT: 800,
    TICK_RATE: 60,
    NPC_RADIUS: 22,
    NPC_SPEED: 2.5,
    VISION_RANGE: 500,
    INTERACTION_RANGE: 60,
    GATE_RADIUS: 30,
    BATCH_INTERVAL: 4000
};

let particleIdCounter = 0;
let feedIdCounter = 0;

const createInitialNPCs = (): NPC[] => {
    const npcData = [
        { name: 'Explorer', x: 150, y: 150 },
        { name: 'Scout', x: 250, y: 150 },
        { name: 'Wanderer', x: CONFIG.WORLD_WIDTH - 150, y: CONFIG.WORLD_HEIGHT - 150 },
        { name: 'Seeker', x: CONFIG.WORLD_WIDTH - 250, y: CONFIG.WORLD_HEIGHT - 150 }
    ];
    const memoryCodes = ['A749', 'B312', 'C856', 'D427'];

    return npcData.map((data, i) => ({
        id: `npc_${i}`,
        name: data.name,
        x: data.x,
        y: data.y,
        targetX: data.x,
        targetY: data.y,
        angle: Math.random() * Math.PI * 2,
        hp: 100,
        energy: 100,
        state: 'idle' as const,
        action: null,
        thought: '',
        team: NPC_TEAM_MAP[data.name],
        trail: [],
        memoryCode: memoryCodes[i]
    }));
};

const createInitialZones = (): Record<string, Zone> => {
    const halfW = CONFIG.WORLD_WIDTH / 2;
    const halfH = CONFIG.WORLD_HEIGHT / 2;
    return {
        start: { id: 'start', name: 'Starting Grounds', theme: 'neutral', bounds: { x: 0, y: 0, w: halfW, h: halfH }, unlocked: true },
        zone_2: { id: 'zone_2', name: 'Crystal Caverns', theme: 'crystal', bounds: { x: halfW, y: 0, w: halfW, h: halfH }, unlocked: false },
        zone_3: { id: 'zone_3', name: 'Whispering Woods', theme: 'forest', bounds: { x: 0, y: halfH, w: halfW, h: halfH }, unlocked: false },
        zone_4: { id: 'zone_4', name: 'The Nexus', theme: 'void', bounds: { x: halfW, y: halfH, w: halfW, h: halfH }, unlocked: false }
    };
};

const createInitialGates = (): Record<string, Gate> => {
    const halfW = CONFIG.WORLD_WIDTH / 2;
    const halfH = CONFIG.WORLD_HEIGHT / 2;
    return {
        gate_1_2: { id: 'gate_1_2', position: [halfW, halfH / 2], unlocked: false, requiresTeamwork: false, toZone: 'zone_2' },
        gate_1_3: { id: 'gate_1_3', position: [halfW / 2, halfH], unlocked: false, requiresTeamwork: true, toZone: 'zone_3' },
        gate_2_4: { id: 'gate_2_4', position: [halfW + halfW / 2, halfH], unlocked: false, requiresTeamwork: false, toZone: 'zone_4' },
        gate_3_4: { id: 'gate_3_4', position: [halfW, halfH + halfH / 2], unlocked: false, requiresTeamwork: true, toZone: 'zone_4' }
    };
};

const createInitialTeams = (): Record<string, Team> => ({
    red: { id: 'red', name: 'Team Red', score: 0, tokens: 50, members: ['Explorer', 'Scout'] },
    blue: { id: 'blue', name: 'Team Blue', score: 0, tokens: 50, members: ['Wanderer', 'Seeker'] }
});

export const useGameStore = create<GameState>((set) => ({
    // Initial state
    running: false,
    tick: 0,
    npcs: createInitialNPCs(),
    zones: createInitialZones(),
    gates: createInitialGates(),
    teams: createInitialTeams(),
    particles: [],
    feedItems: [],
    commentary: '',
    hoveredEntity: null,
    mousePos: { x: 0, y: 0 },
    connected: false,
    slmProvider: '--',
    brainProvider: '--',

    // Actions
    setRunning: (running) => set({ running }),
    setTick: (tick) => set({ tick }),

    updateNPC: (id, updates) => set((state) => ({
        npcs: state.npcs.map(npc => npc.id === id ? { ...npc, ...updates } : npc)
    })),

    setNPCs: (npcs) => set({ npcs }),
    setZones: (zones) => set({ zones }),
    setGates: (gates) => set({ gates }),
    setTeams: (teams) => set({ teams }),

    updateTeamScore: (teamId, score) => set((state) => ({
        teams: {
            ...state.teams,
            [teamId]: { ...state.teams[teamId], score }
        }
    })),

    addParticle: (particle) => set((state) => ({
        particles: [...state.particles, { ...particle, id: ++particleIdCounter }]
    })),

    updateParticles: () => set((state) => ({
        particles: state.particles
            .map(p => ({
                ...p,
                x: p.x + p.vx,
                y: p.y + p.vy,
                life: p.life - 1
            }))
            .filter(p => p.life > 0)
    })),

    addFeedItem: (item) => set((state) => ({
        feedItems: [
            { ...item, id: ++feedIdCounter, timestamp: Date.now() },
            ...state.feedItems.slice(0, 19) // Keep max 20 items
        ]
    })),

    setCommentary: (commentary) => set({ commentary }),
    setHoveredEntity: (hoveredEntity) => set({ hoveredEntity }),
    setMousePos: (mousePos) => set({ mousePos }),
    setConnected: (connected) => set({ connected }),
    setProviders: (slmProvider, brainProvider) => set({ slmProvider, brainProvider }),

    reset: () => set({
        running: false,
        tick: 0,
        npcs: createInitialNPCs(),
        zones: createInitialZones(),
        gates: createInitialGates(),
        teams: createInitialTeams(),
        particles: [],
        feedItems: [],
        commentary: ''
    })
}));
