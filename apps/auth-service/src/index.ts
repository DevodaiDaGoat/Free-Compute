import express from 'express';
import helmet from 'helmet';
import { authRouter } from './routes/auth';
import { sessionRouter } from './routes/session';
import { errorHandler } from './middleware/errorHandler';

const app = express();
const PORT = process.env.PORT || 3001;

// Security middleware
app.use(helmet());
app.use(express.json({ limit: '1mb' })); // Limit request body size

// Health check
app.get('/health', (_req, res) => {
  res.json({ status: 'ok' });
});

// Routes
app.use('/auth', authRouter);
app.use('/session', sessionRouter);

// Global error handler — never leaks internal details
app.use(errorHandler);

app.listen(PORT, () => {
  console.log(`auth-service listening on port ${PORT}`);
});

export default app;
