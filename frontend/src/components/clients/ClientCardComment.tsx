type ClientCardCommentProps = {
  comment?: string;
  className?: string;
};

export default function ClientCardComment({ comment, className = 'client-card-comment' }: ClientCardCommentProps) {
  if (!comment) return null;

  return (
    <span className={className} title={comment}>
      {comment}
    </span>
  );
}