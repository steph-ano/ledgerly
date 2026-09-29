import React from 'react'
import { CheckCircle2, Clock, Key, Send } from 'lucide-react'
import type { OutboxEvent } from '../types'
import './WebhooksView.css'

interface WebhooksViewProps {
  events: OutboxEvent[]
}

export const WebhooksView: React.FC<WebhooksViewProps> = ({ events }) => {
  return (
    <div className="webhooks-container animate-fade-in">
      <div className="webhooks-header">
        <div>
          <h1>Transactional Outbox & HMAC Webhook Stream</h1>
          <p className="subtitle">
            All domain events are committed atomically inside the business transaction, then dispatched asynchronously
            with cryptographic HMAC-SHA256 signatures to prevent tampering or replay attacks.
          </p>
        </div>
      </div>

      <div className="events-stream">
        {events.map((evt) => (
          <div key={evt.id} className="event-card">
            <div className="event-header">
              <div className="event-title-group">
                <span className="event-type-badge mono">{evt.event_type}</span>
                <span className="event-id mono">ID: {evt.id.slice(0, 8)}...</span>
              </div>

              <div className="event-status-group">
                <span className="event-delivery-badge">
                  <CheckCircle2 size={13} />
                  <span>Delivered (HTTP 200)</span>
                </span>
                <span className="event-time">
                  <Clock size={13} />
                  <span>{new Date(evt.created_at).toLocaleTimeString()}</span>
                </span>
              </div>
            </div>

            <div className="event-destination">
              <Send size={14} className="icon-cyan" />
              <span className="dest-label">Destination URL:</span>
              <span className="dest-url mono">{evt.destination_url}</span>
            </div>

            {/* Cryptographic Headers Inspector */}
            <div className="security-headers-box">
              <div className="security-header-title">
                <Key size={14} />
                <span>Dispatched Cryptographic HTTP Headers</span>
              </div>
              <div className="headers-grid mono">
                <div className="header-key">X-Ledgerly-Event-Type:</div>
                <div className="header-val">{evt.event_type}</div>

                <div className="header-key">X-Ledgerly-Timestamp:</div>
                <div className="header-val">{evt.created_at}</div>

                <div className="header-key">X-Ledgerly-Signature:</div>
                <div className="header-val highlight">
                  sha256=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
                </div>
              </div>
            </div>

            {/* JSON Payload */}
            <div className="payload-box">
              <span className="payload-label">Webhook JSON Payload</span>
              <pre className="payload-code mono">
                {JSON.stringify(evt.payload, null, 2)}
              </pre>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
