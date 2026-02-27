import { describe, it, expect, beforeAll } from 'vitest';
import path from 'node:path';
import request from 'supertest';
import type { Express } from 'express';
import { createApp } from '../src/api/app';
import { ProblemStore } from '../src/problems/store';

let app: Express;

beforeAll(async () => {
  const problems = await ProblemStore.fromDir(path.resolve(__dirname, '../problems'));
  app = createApp({ problems }).app;
});

describe('HTTP API (Supertest)', () => {
  it('GET /api/health', async () => {
    const res = await request(app).get('/api/health');
    expect(res.status).toBe(200);
    expect(res.body.status).toBe('ok');
  });

  it('GET /api/languages lists python + javascript', async () => {
    const res = await request(app).get('/api/languages');
    expect(res.status).toBe(200);
    const ids = res.body.languages.map((l: { id: string }) => l.id);
    expect(ids).toContain('python');
    expect(ids).toContain('javascript');
  });

  it('POST /api/run executes Python and returns stdout', async () => {
    const res = await request(app)
      .post('/api/run')
      .send({ language: 'python', source: 'print("hi from api")\n' });
    expect(res.status).toBe(200);
    expect(res.body.status).toBe('ok');
    expect(res.body.stdout.replace(/\r\n/g, '\n')).toBe('hi from api\n');
  });

  it('POST /api/run executes Node and returns stdout', async () => {
    const res = await request(app)
      .post('/api/run')
      .send({ language: 'javascript', source: 'console.log(40 + 2)\n' });
    expect(res.status).toBe(200);
    expect(res.body.stdout.replace(/\r\n/g, '\n')).toBe('42\n');
  });

  it('POST /api/run with empty source -> 400 validation', async () => {
    const res = await request(app).post('/api/run').send({ language: 'python', source: '' });
    expect(res.status).toBe(400);
    expect(res.body.error.type).toBe('validation');
  });

  it('POST /api/run with unknown language -> 400', async () => {
    const res = await request(app).post('/api/run').send({ language: 'ruby', source: 'x' });
    expect(res.status).toBe(400);
  });

  it('POST /api/run with oversized source -> 413', async () => {
    const big = 'x'.repeat(200 * 1024);
    const res = await request(app).post('/api/run').send({ language: 'python', source: big });
    expect(res.status).toBe(413);
  });

  it('GET /api/problems lists loaded problems', async () => {
    const res = await request(app).get('/api/problems');
    expect(res.status).toBe(200);
    const ids = res.body.problems.map((p: { id: string }) => p.id);
    expect(ids).toContain('sum-of-ints');
  });

  it('GET /api/problems/:id strips hidden test cases', async () => {
    const res = await request(app).get('/api/problems/sum-of-ints');
    expect(res.status).toBe(200);
    expect(res.body.publicTestCaseNames).not.toContain('negatives');
    expect(JSON.stringify(res.body)).not.toContain('expectedStdout');
    expect(res.body.totalCases).toBeGreaterThan(res.body.publicTestCaseNames.length);
  });

  it('GET unknown problem -> 404', async () => {
    const res = await request(app).get('/api/problems/does-not-exist');
    expect(res.status).toBe(404);
  });

  it('POST /api/problems/:id/submit grades a correct solution', async () => {
    const source = 'import sys\nprint(sum(int(x) for x in sys.stdin.read().split()))\n';
    const res = await request(app)
      .post('/api/problems/sum-of-ints/submit')
      .send({ language: 'python', source });
    expect(res.status).toBe(200);
    expect(res.body.passed).toBe(true);
    expect(res.body.passedCases).toBe(res.body.totalCases);
    // hidden cases present but redacted
    const hidden = res.body.results.find((r: { hidden: boolean }) => r.hidden);
    expect(hidden).toBeDefined();
    expect(hidden.expected).toBeUndefined();
  });

  it('POST submit with a wrong solution -> passed:false, partial count', async () => {
    const res = await request(app)
      .post('/api/problems/sum-of-ints/submit')
      .send({ language: 'python', source: 'print(0)\n' });
    expect(res.status).toBe(200);
    expect(res.body.passed).toBe(false);
    expect(res.body.passedCases).toBeLessThan(res.body.totalCases);
  });
});
