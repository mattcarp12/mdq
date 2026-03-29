import React, { useEffect, useState } from 'react';
import axios from 'axios';
import { useAuth } from './AuthContext';
import api from './api';

interface Job {
  id: string;
  type: string;
  payload: any;
  status: string;
  created_at: string;
}

export const Dashboard = () => {
  const { token, logout } = useAuth();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [jobType, setJobType] = useState('video_compression');
  const [payloadData, setPayloadData] = useState('{\n  "file": "vacation.mp4",\n  "quality": "1080p"\n}');
  const [error, setError] = useState('');

  const fetchJobs = async () => {
    try {
      const response = await api.get('/v1/jobs');   // ← no manual headers needed anymore!
      setJobs(response.data.data);
    } catch (err) {
      console.error('Failed to fetch jobs', err);
    }
  };

  // Fetch jobs on mount and setup polling
  useEffect(() => {
    fetchJobs();
    const interval = setInterval(fetchJobs, 5000); // Poll every 5 seconds
    return () => clearInterval(interval); // Cleanup on unmount
  }, [token]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    let parsedPayload;
    try {
      parsedPayload = JSON.parse(payloadData);
    } catch (err) {
      setError('Invalid JSON format in the payload field.');
      return;
    }

    try {
      await api.post('/v1/jobs', {   // ← no manual headers needed
        type: jobType,
        payload: parsedPayload,
        max_retries: 3,
      });

      // Reset form and immediately fetch updated list
      setJobType('video_compression');
      fetchJobs();
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to submit job to the queue.');
    }
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>MDQ Control Panel</h2>
        <button onClick={logout}>Log Out</button>
      </div>

      <fieldset style={{ marginBottom: '20px', border: '2px outset #ffffff', backgroundColor: '#e0e0e0' }}>
        <legend style={{ fontWeight: 'bold', backgroundColor: '#c0c0c0', padding: '0 5px' }}>Submit New Job</legend>

        {error && <div className="error-message">{error}</div>}

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: '10px' }}>
            <label><strong>Job Type:</strong></label><br />
            <input
              type="text"
              value={jobType}
              onChange={(e) => setJobType(e.target.value)}
              required
            />
          </div>
          <div style={{ marginBottom: '10px' }}>
            <label><strong>Payload (JSON):</strong></label><br />
            <textarea
              value={payloadData}
              onChange={(e) => setPayloadData(e.target.value)}
              rows={4}
              style={{ width: '100%', fontFamily: 'Courier New', border: '2px inset #ffffff' }}
              required
            />
          </div>
          <button type="submit">Enqueue Task</button>
        </form>
      </fieldset>

      <h3>Job Queue History</h3>
      {jobs.length === 0 ? (
        <p><i>No jobs found in the database.</i></p>
      ) : (
        <table style={{ width: '100%', borderCollapse: 'collapse', backgroundColor: '#ffffff', border: '2px solid #000000' }}>
          <thead>
            <tr style={{ backgroundColor: '#000080', color: '#ffffff', textAlign: 'left' }}>
              <th style={{ padding: '5px', border: '1px solid #000000' }}>ID (UUID)</th>
              <th style={{ padding: '5px', border: '1px solid #000000' }}>Type</th>
              <th style={{ padding: '5px', border: '1px solid #000000' }}>Status</th>
              <th style={{ padding: '5px', border: '1px solid #000000' }}>Submitted At</th>
            </tr>
          </thead>
          <tbody>
            {jobs.map((job) => (
              <tr key={job.id}>
                <td style={{ padding: '5px', border: '1px solid #c0c0c0', fontFamily: 'Courier New', fontSize: '0.85em' }}>
                  {job.id}
                </td>
                <td style={{ padding: '5px', border: '1px solid #c0c0c0' }}>{job.type}</td>
                <td style={{ padding: '5px', border: '1px solid #c0c0c0', fontWeight: 'bold' }}>
                  {/* SOTA Tip: Color coding status makes dashboards instantly readable */}
                  <span style={{
                    color: job.status === 'COMPLETED' ? 'green' :
                      job.status === 'FAILED' ? 'red' :
                        job.status === 'RUNNING' ? 'blue' : 'black'
                  }}>
                    {job.status}
                  </span>
                </td>
                <td style={{ padding: '5px', border: '1px solid #c0c0c0', fontSize: '0.9em' }}>
                  {new Date(job.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
};