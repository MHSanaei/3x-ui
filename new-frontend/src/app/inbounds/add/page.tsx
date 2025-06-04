"use client";
import React, { useState } from 'react';
import InboundForm from '@/components/inbounds/InboundForm';
import { Inbound } from '@/types/inbound';
import { post } from '@/services/api';
import { useRouter } from 'next/navigation';

const AddInboundPage: React.FC = () => {
  const [formLoading, setFormLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  const handleSubmit = async (inboundData: Partial<Inbound>) => {
    setFormLoading(true);
    setError(null);
    try {
      const response = await post<Inbound>('/inbound/add', inboundData);
      if (response.success && response.data) {
        router.push('/inbounds');
      } else {
        setError(response.message || 'Failed to create inbound.');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An unknown error occurred.');
    } finally {
      setFormLoading(false);
    }
  };

  return (
    <div className="p-2 md:p-0 max-w-4xl mx-auto">
      <h1 className="text-2xl md:text-3xl font-semibold mb-6 text-gray-800 dark:text-gray-200">Add New Inbound</h1>
      {error && (
        <div className="mb-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">
          {error}
        </div>
      )}
      <InboundForm onSubmitForm={handleSubmit} formLoading={formLoading} isEditMode={false} />
    </div>
  );
};
export default AddInboundPage;
