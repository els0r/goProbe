import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './components/App';
import './index.css';

class RootErrorBoundary extends React.Component<{ children: React.ReactNode }, { error?: Error }> {
  constructor(props: { children: React.ReactNode }) {
    super(props)
    this.state = { error: undefined }
  }
  static getDerivedStateFromError(error: Error) { return { error } }
  componentDidCatch(error: Error, info: any) { console.error('RootErrorBoundary caught', error, info) }
  render() {
    if (this.state.error) {
      return <div style={{ padding: 16, fontFamily: 'monospace', color: '#f88' }}>Runtime error: {this.state.error.message}</div>
    }
    return this.props.children
  }
}

// helpful startup log to debug blank screen issues
// eslint-disable-next-line no-console
console.log('goquery-ui starting, GQ_API_BASE_URL=', process.env.GQ_API_BASE_URL)

const container = document.getElementById('root');
if (container) {
  const root = createRoot(container);
  root.render(<RootErrorBoundary><App /></RootErrorBoundary>);
} else {
  // eslint-disable-next-line no-console
  console.error('#root element not found in document')
}
