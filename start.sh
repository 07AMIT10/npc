#!/bin/bash

# Smart NPC Explorer - Quick Start Script

echo "🧭 Smart NPC Explorer - Setup"
echo "=============================="

# Load .env file if it exists
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | grep -v '^$' | xargs)
fi

# Check for required environment variables
check_api_keys() {
    local has_slm=false
    local has_brain=false
    
    if [ -n "$GROQ_API_KEY" ]; then
        echo "✓ Groq API key found (model: ${GROQ_MODEL:-llama-3.1-8b-instant})"
        has_slm=true
    fi
    if [ -n "$SAMBANOVA_API_KEY" ]; then
        echo "✓ SambaNova API key found (model: ${SAMBANOVA_MODEL:-Meta-Llama-3.1-8B-Instruct})"
        has_slm=true
    fi
    if [ -n "$OPENROUTER_API_KEY" ]; then
        echo "✓ OpenRouter API key found (model: ${OPENROUTER_MODEL:-meta-llama/llama-3.1-8b-instruct})"
        has_slm=true
    fi
    if [ -n "$HF_API_KEY" ]; then
        echo "✓ HuggingFace API key found (model: ${HF_MODEL:-meta-llama/Llama-3.2-3B-Instruct})"
        has_slm=true
    fi
    if [ -n "$GEMINI_API_KEY" ]; then
        echo "✓ Gemini API key found (model: ${GEMINI_MODEL:-gemini-2.0-flash})"
        has_brain=true
    fi
    if [ -n "$OPENAI_API_KEY" ]; then
        echo "✓ OpenAI API key found (model: ${OPENAI_MODEL:-gpt-4o-mini})"
        has_brain=true
    fi
    
    if [ "$has_slm" = false ] && [ "$has_brain" = false ]; then
        echo ""
        echo "⚠️  No API keys found - running in DEMO mode"
        echo "   NPCs will use simple exploration behavior"
        echo ""
        echo "Add your API keys to .env file:"
        echo "  GROQ_API_KEY=your_key        # Fast SLM"
        echo "  GEMINI_API_KEY=your_key      # Brain LLM"
        echo ""
    fi
}

# Main
check_api_keys

echo ""
echo "Starting server..."

# Rebuild if needed
if [ ! -f npc-server ] || [ cmd/server/main.go -nt npc-server ]; then
    echo "Rebuilding..."
    go build -o npc-server ./cmd/server
fi

# Run
./npc-server
