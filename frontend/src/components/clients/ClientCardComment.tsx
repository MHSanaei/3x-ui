type ClientCardCommentProps = {
  comment?: string;
};

export default function ClientCardComment({ comment }: ClientCardCommentProps) {
  if (!comment) return null;

  return (
    <div className="client-card-comment" title={comment}>
      {comment}
    </div>
  );
}