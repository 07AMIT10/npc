import { useEffect, useRef } from 'react';
import { useGameStore, CONFIG } from './store/gameStore';
import { useWebSocket } from './hooks/useWebSocket';
import { GameCanvas } from './components/GameCanvas';
import { Header } from './components/Header';
import { Sidebar } from './components/Sidebar';
import { Tooltip, Minimap } from './components/Overlay';
import './App.css';

function App() {
  const { running, tick, setTick } = useGameStore();
  const { requestBatchDecisions, requestCommentary } = useWebSocket();
  const lastBatchTime = useRef(0);
  const lastCommentaryTime = useRef(0);

  // Game loop - request batch decisions periodically
  useEffect(() => {
    if (!running) return;

    const interval = setInterval(() => {
      const now = Date.now();

      // Batch decisions every 4 seconds
      if (now - lastBatchTime.current > CONFIG.BATCH_INTERVAL) {
        requestBatchDecisions();
        lastBatchTime.current = now;
      }

      // Commentary every 10 seconds
      if (now - lastCommentaryTime.current > 10000) {
        requestCommentary();
        lastCommentaryTime.current = now;
      }

      setTick(tick + 1);
    }, 1000 / CONFIG.TICK_RATE);

    return () => clearInterval(interval);
  }, [running, tick, setTick, requestBatchDecisions, requestCommentary]);

  return (
    <div className="app">
      <Header />

      <main className="main-content">
        <div className="canvas-container">
          <GameCanvas />
          <Minimap />
        </div>
        <Sidebar />
      </main>

      <Tooltip />
    </div>
  );
}

export default App;
