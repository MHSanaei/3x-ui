"use client";
import React, { useState, useEffect, useCallback } from 'react';
import InboundForm from '@/components/inbounds/InboundForm';
import { Inbound } from '@/types/inbound';
import { post } from '@/services/api';
import { useRouter } from 'next/navigation'; // Removed useParams, id comes from prop

interface EditInboundClientComponentProps {
  id: string; // Passed from the server component page.tsx
}

const EditInboundClientComponent: React.FC<EditInboundClientComponentProps> = ({ id }) => {
  const [pageLoading, setPageLoading] = useState(true);
  const [formProcessing, setFormProcessing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [initialInboundData, setInitialInboundData] = useState<Inbound | undefined>(undefined);
  const router = useRouter();
  // const params = useParams(); // Not needed, id comes from props
  // const id = params.id as string; // Not needed

  const fetchInboundData = useCallback(async () => {
    if (!id) { // id prop should always be present
        setError("Inbound ID is missing.");
        setPageLoading(false);
        return;
    }
    setPageLoading(true);
    setError(null);
    try {
      const response = await post<Inbound[]>('/inbound/list', {});
      if (response.success && response.data) {
        const numericId = parseInt(id, 10);
        const inbound = response.data.find(ib => ib.id === numericId);
        if (inbound) {
          setInitialInboundData(inbound);
        } else {
          setError('Inbound not found.');
        }
      } else {
        setError(response.message || 'Failed to fetch inbound data.');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An unknown error occurred while fetching data.');
    } finally {
      setPageLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchInboundData();
  }, [fetchInboundData]);

  const handleSubmit = async (inboundData: Partial<Inbound>) => {
    setFormProcessing(true);
    setError(null);
    try {
      const response = await post<Inbound>(`/inbound/update/${id}`, inboundData);
      if (response.success) {
        router.push('/inbounds');
      } else {
        setError(response.message || 'Failed to update inbound.');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An unknown error occurred.');
    } finally {
      setFormProcessing(false);
    }
  };

  if (pageLoading) {
    return <div className="p-4 text-center text-gray-700 dark:text-gray-300">Loading inbound data...</div>;
  }
  if (error && !initialInboundData) {
     return <div className="p-4 text-red-500 dark:text-red-400 text-center">Error: {error}</div>;
  }
  if (!initialInboundData && !pageLoading) {
    return <div className="p-4 text-center text-gray-700 dark:text-gray-300">Inbound not found.</div>;
  }

  return (
    <div className="p-2 md:p-0 max-w-4xl mx-auto">
      <h1 className="text-2xl md:text-3xl font-semibold mb-6 text-gray-800 dark:text-gray-200">Edit Inbound: {initialInboundData?.remark || id}</h1>
      {error && initialInboundData && (
        <div className="mb-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">
          {error}
        </div>
      )}
      {initialInboundData && <InboundForm
        initialData={initialInboundData}
        onSubmitForm={handleSubmit}
        formLoading={formProcessing}
        isEditMode={true}
      />}
    </div>
  );
};
export default EditInboundClientComponent;
