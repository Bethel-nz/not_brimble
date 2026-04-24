const http = require('http');

const html = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sample Service | Not Brimble</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        @import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap');
        body { background-color: #050505; color: #ffffff; font-family: 'JetBrains Mono', monospace; }
        .gradient-bg {
            background: radial-gradient(circle at 50% 50%, #1a1a1a 0%, #050505 100%);
        }
    </style>
</head>
<body class="gradient-bg min-h-screen flex flex-col items-center justify-center p-6">
    <div class="max-w-2xl w-full border border-zinc-800 bg-zinc-900/50 p-12 rounded-2xl backdrop-blur-xl shadow-2xl">
        <div class="flex items-center gap-4 mb-8">
            <div class="w-12 h-12 bg-white flex items-center justify-center rounded-lg shadow-[0_0_30px_rgba(255,255,255,0.2)]">
                <span class="text-black text-2xl font-bold">▲</span>
            </div>
            <div>
                <h1 class="text-2xl font-bold tracking-tighter">SERVICE_READY</h1>
                <p class="text-zinc-500 text-xs uppercase tracking-[0.3em]">Deployment Successful</p>
            </div>
        </div>

        <div class="space-y-6">
            <div class="p-4 bg-zinc-950/50 border border-zinc-800 rounded-lg">
                <p class="text-zinc-400 text-xs mb-2 uppercase font-bold opacity-50">Instance Metadata</p>
                <div class="grid grid-cols-2 gap-4">
                    <div>
                        <span class="block text-[10px] text-zinc-600 uppercase">Region</span>
                        <span class="text-sm">us-east-1</span>
                    </div>
                    <div>
                        <span class="block text-[10px] text-zinc-600 uppercase">Runtime</span>
                        <span class="text-sm">Node.js 20.x</span>
                    </div>
                </div>
            </div>

            <div class="flex items-center justify-between pt-4">
                <div class="flex gap-2">
                    <span class="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
                    <span class="text-[10px] text-green-500 font-bold uppercase tracking-widest">Traffic Active</span>
                </div>
                <div class="text-[10px] text-zinc-600 uppercase font-bold">
                    Powered by Railpack
                </div>
            </div>
        </div>
    </div>

    <footer class="mt-12 text-zinc-700 text-[10px] font-bold uppercase tracking-[0.4em]">
        not_brimble // internal_testing_suite
    </footer>
</body>
</html>
`;

const server = http.createServer((req, res) => {
  res.statusCode = 200;
  res.setHeader('Content-Type', 'text/html');
  res.end(html);
});

const port = process.env.PORT || 3000;
server.listen(port, () => {
  console.log(`Sample app running on port ${port}`);
});
