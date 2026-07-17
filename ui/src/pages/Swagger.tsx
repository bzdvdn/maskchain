export function Swagger() {
  return (
    <div className="card" style={{ textAlign: 'center', padding: '60px 20px' }}>
      <div style={{ fontSize: 48, opacity: 0.3, marginBottom: 16 }}>⛁</div>
      <h3>Swagger / OpenAPI</h3>
      <p style={{ color: 'var(--color-text-muted)', marginTop: 8 }}>
        API documentation available at
      </p>
      <p style={{ marginTop: 12 }}>
        <a href="/api/v1/docs" target="_blank" rel="noopener noreferrer" className="btn">
          Open Swagger UI
        </a>
      </p>
      <p style={{ marginTop: 8 }}>
        <code style={{ fontSize: 13 }}>GET /api/v1/openapi.yaml</code>
      </p>
    </div>
  )
}
