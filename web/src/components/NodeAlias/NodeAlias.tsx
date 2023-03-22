interface NodeAliasProps {
  alias?: string;
  nodeLimit: NodeLimit;
}

const NodeAlias = ({ alias, nodeLimit }: NodeAliasProps) => (
  <>
    {alias || `${nodeLimit.node.slice(0, 8)}...${nodeLimit.node.slice(58, 66)}`}
  </>
);

export default NodeAlias;
