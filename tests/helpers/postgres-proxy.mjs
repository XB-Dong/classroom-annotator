import net from 'node:net';

// A test-owned loopback relay lets one product process lose DB connectivity
// without stopping the shared forward or touching another database/client.
export async function postgresProxy(connectionString) {
  const target = new URL(connectionString);
  const upstreamHost = target.hostname;
  const upstreamPort = Number(target.port || 5432);
  const sockets = new Set();
  let available = true;
  let active = 0;
  let peak = 0;
  const server = net.createServer(client => {
    if (!available) { client.destroy(); return; }
    const upstream = net.connect(upstreamPort, upstreamHost);
    sockets.add(client); sockets.add(upstream);
    active++; peak = Math.max(peak, active);
    client.once('close', () => { active--; sockets.delete(client); upstream.destroy(); });
    upstream.once('close', () => { sockets.delete(upstream); client.destroy(); });
    client.on('error', () => upstream.destroy());
    upstream.on('error', () => client.destroy());
    client.pipe(upstream); upstream.pipe(client);
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject); server.listen(0, '127.0.0.1', resolve);
  });
  target.hostname = '127.0.0.1'; target.port = String(server.address().port);
  return {
    connectionString: target.toString(),
    stats: () => ({ active, peak }),
    disconnect: () => { available = false; for (const socket of sockets) socket.destroy(); },
    reconnect: () => { available = true; },
    stop: async () => { available = false; for (const socket of sockets) socket.destroy(); await new Promise(resolve => server.close(resolve)); },
  };
}
