'use client';

import { useState, useEffect, useRef, useCallback } from 'react';

interface StoredMessage {
  content: string;
  is_sender: boolean;
  timestamp: number;
}

interface Message {
  type: string;
  content?: string;
  timestamp?: number;
  latitude?: number;
  longitude?: number;
  device_id?: string;
  messages?: StoredMessage[];
}

interface ChatMessage {
  id: string;
  type: 'sent' | 'received' | 'system';
  content: string;
  timestamp: number;
}

type ConnectionStatus = 'disconnected' | 'searching' | 'connected';

// Get or create device ID
function getDeviceId(): string {
  if (typeof window === 'undefined') return '';

  let deviceId = localStorage.getItem('wootalk_device_id');
  if (!deviceId) {
    deviceId = crypto.randomUUID();
    localStorage.setItem('wootalk_device_id', deviceId);
  }
  return deviceId;
}

export default function Home() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [inputValue, setInputValue] = useState('');
  const [status, setStatus] = useState<ConnectionStatus>('disconnected');
  const [isTyping, setIsTyping] = useState(false);
  const [location, setLocation] = useState<{ lat: number; lng: number } | null>(null);
  const [locationError, setLocationError] = useState<string | null>(null);
  const [distanceInfo, setDistanceInfo] = useState<string>('');
  const [deviceId, setDeviceId] = useState<string>('');

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const typingTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  // Initialize device ID
  useEffect(() => {
    setDeviceId(getDeviceId());
  }, []);

  // Get user's location
  useEffect(() => {
    if ('geolocation' in navigator) {
      navigator.geolocation.getCurrentPosition(
        (position) => {
          setLocation({
            lat: position.coords.latitude,
            lng: position.coords.longitude,
          });
          setLocationError(null);
        },
        (error) => {
          console.error('Geolocation error:', error);
          setLocationError('Location access denied. Using default location.');
          setLocation({ lat: 0, lng: 0 });
        },
        {
          enableHighAccuracy: true,
          timeout: 10000,
          maximumAge: 0,
        }
      );
    } else {
      setLocationError('Geolocation not supported by your browser.');
      setLocation({ lat: 0, lng: 0 });
    }
  }, []);

  // Initialize WebSocket connection
  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8080/ws');
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
      const message: Message = JSON.parse(event.data);

      switch (message.type) {
        case 'matched':
          setStatus('connected');
          setDistanceInfo(message.content || '');
          addSystemMessage('Connected to a stranger! Say hi!');
          break;

        case 'message':
          addMessage('received', message.content || '');
          break;

        case 'partner_left':
          setStatus('disconnected');
          setDistanceInfo('');
          addSystemMessage('Stranger has disconnected.');
          break;

        case 'searching':
          setStatus('searching');
          addSystemMessage('Looking for someone to chat with...');
          break;

        case 'typing':
          setIsTyping(true);
          break;

        case 'stop_typing':
          setIsTyping(false);
          break;

        case 'error':
          addSystemMessage(`Error: ${message.content}`);
          break;

        case 'restore_session':
          // Restore previous chat session
          setStatus('connected');
          if (message.messages && message.messages.length > 0) {
            const restoredMessages: ChatMessage[] = message.messages.map((msg, index) => ({
              id: `restored-${index}-${msg.timestamp}`,
              type: msg.is_sender ? 'sent' : 'received',
              content: msg.content,
              timestamp: msg.timestamp,
            }));
            setMessages(restoredMessages);
          }
          addSystemMessage(message.content || 'Reconnected to previous chat');
          break;

        case 'session_expired':
          addSystemMessage(message.content || 'Previous session expired');
          break;
      }
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      setStatus('disconnected');
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    return () => {
      ws.close();
    };
  }, []);

  const addMessage = (type: 'sent' | 'received', content: string) => {
    const newMessage: ChatMessage = {
      id: Date.now().toString() + Math.random().toString(36),
      type,
      content,
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, newMessage]);
    setIsTyping(false);
  };

  const addSystemMessage = (content: string) => {
    const newMessage: ChatMessage = {
      id: Date.now().toString() + Math.random().toString(36),
      type: 'system',
      content,
      timestamp: Date.now(),
    };
    setMessages((prev) => [...prev, newMessage]);
  };

  const sendMessage = (e: React.FormEvent) => {
    e.preventDefault();

    if (!inputValue.trim() || status !== 'connected' || !wsRef.current) {
      return;
    }

    const message: Message = {
      type: 'message',
      content: inputValue.trim(),
    };

    wsRef.current.send(JSON.stringify(message));
    addMessage('sent', inputValue.trim());
    setInputValue('');

    // Stop typing indicator
    if (wsRef.current) {
      wsRef.current.send(JSON.stringify({ type: 'stop_typing' }));
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setInputValue(e.target.value);

    if (!wsRef.current || status !== 'connected') return;

    // Send typing indicator
    wsRef.current.send(JSON.stringify({ type: 'typing' }));

    // Clear previous timeout
    if (typingTimeoutRef.current) {
      clearTimeout(typingTimeoutRef.current);
    }

    // Stop typing after 2 seconds of inactivity
    typingTimeoutRef.current = setTimeout(() => {
      if (wsRef.current) {
        wsRef.current.send(JSON.stringify({ type: 'stop_typing' }));
      }
    }, 2000);
  };

  const findPartner = useCallback(() => {
    if (!wsRef.current || !location || !deviceId) return;

    setMessages([]);
    setDistanceInfo('');

    const message: Message = {
      type: 'connect',
      latitude: location.lat,
      longitude: location.lng,
      device_id: deviceId,
    };

    wsRef.current.send(JSON.stringify(message));
  }, [location, deviceId]);

  const disconnect = () => {
    if (!wsRef.current) return;

    const message: Message = {
      type: 'disconnect',
    };

    wsRef.current.send(JSON.stringify(message));
    setStatus('disconnected');
    setDistanceInfo('');
    addSystemMessage('You disconnected from the chat.');
  };

  const newPartner = () => {
    disconnect();
    setTimeout(() => {
      findPartner();
    }, 100);
  };

  const getStatusText = () => {
    switch (status) {
      case 'connected':
        return distanceInfo ? `Connected - ${distanceInfo}` : 'Connected';
      case 'searching':
        return 'Searching for someone...';
      default:
        return 'Disconnected';
    }
  };

  return (
    <div className="container">
      <header className="header">
        <h1>WooTalk</h1>
        <p>Anonymous chat with people nearby</p>
      </header>

      <div className="chat-container">
        <div className="status-bar">
          <div className="status">
            <span className={`status-dot ${status}`}></span>
            <span>{getStatusText()}</span>
          </div>
          <div className="controls">
            {status === 'disconnected' && (
              <button
                className="btn btn-success"
                onClick={findPartner}
                disabled={!location || !deviceId}
              >
                Start Chat
              </button>
            )}
            {status === 'searching' && (
              <button className="btn btn-danger" onClick={disconnect}>
                Cancel
              </button>
            )}
            {status === 'connected' && (
              <>
                <button className="btn btn-primary" onClick={newPartner}>
                  New Partner
                </button>
                <button className="btn btn-danger" onClick={disconnect}>
                  Disconnect
                </button>
              </>
            )}
          </div>
        </div>

        {status === 'disconnected' && messages.length === 0 ? (
          <div className="welcome-screen">
            <h2>Welcome!</h2>
            <p>
              Click "Start Chat" to connect with a random stranger nearby.
              Your conversations can be resumed if you return within 30 minutes.
            </p>
            {locationError && (
              <p className="location-status error">{locationError}</p>
            )}
            {location && !locationError && (
              <p className="location-status">
                Location acquired. Ready to connect!
              </p>
            )}
          </div>
        ) : (
          <>
            <div className="messages">
              {messages.map((msg) => (
                <div key={msg.id} className={`message ${msg.type}`}>
                  {msg.content}
                </div>
              ))}
              {isTyping && (
                <div className="typing-indicator">
                  <span></span>
                  <span></span>
                  <span></span>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>

            <form className="input-area" onSubmit={sendMessage}>
              <input
                type="text"
                value={inputValue}
                onChange={handleInputChange}
                placeholder={
                  status === 'connected'
                    ? 'Type a message...'
                    : 'Waiting for connection...'
                }
                disabled={status !== 'connected'}
              />
              <button type="submit" disabled={status !== 'connected' || !inputValue.trim()}>
                Send
              </button>
            </form>
          </>
        )}
      </div>
    </div>
  );
}
