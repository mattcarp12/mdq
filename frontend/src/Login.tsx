import React, { useState } from 'react';
import { useAuth } from './AuthContext';
import { useNavigate } from 'react-router-dom';
import api from './api';

export const Login = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    try {
      const response = await api.post('/v1/login', {
        email,
        password,
      });

      login(response.data.token);
      navigate('/dashboard');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to login. Please try again.');
    }
  };

  return (
    <div>
      <h2>MDQ System Login</h2>
      <p><i>Authorized Personnel Only</i></p>

      {error && <div className="error-message">ERROR: {error}</div>}

      <form onSubmit={handleLogin} style={{ maxWidth: '300px' }}>
        <div>
          <label><strong>Email Address:</strong></label>
          <input
            type="text"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div>
          <label><strong>Password:</strong></label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        <button type="submit">Execute Login</button>
      </form>
    </div>
  );
};