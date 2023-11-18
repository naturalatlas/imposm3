package binary

import (
	"time"

	osm "github.com/omniscale/go-osm"
)

const coordFactor float64 = 11930464.7083 // ((2<<31)-1)/360.0

func CoordToInt(coord float64) uint32 {
	return uint32((coord + 180.0) * coordFactor)
}

func IntToCoord(coord uint32) float64 {
	return float64((float64(coord) / coordFactor) - 180.0)
}

func MarshalNode(node *osm.Node) ([]byte, error) {
	pbfNode := &Node{}
	pbfNode.fromWgsCoord(node.Long, node.Lat)
	pbfNode.Tags = tagsAsArray(node.Tags)
	if node.Metadata != nil {
		pbfNode.Timestamp = node.Metadata.Timestamp.Unix();
	}
	return pbfNode.Marshal()
}

func UnmarshalNode(data []byte) (node *osm.Node, err error) {
	pbfNode := &Node{}
	err = pbfNode.Unmarshal(data)
	if err != nil {
		return nil, err
	}

	node = &osm.Node{}
	node.Long, node.Lat = pbfNode.wgsCoord()
	node.Tags = tagsFromArray(pbfNode.Tags)
	node.Metadata = &osm.Metadata{
		Timestamp: time.Unix(pbfNode.Timestamp, 0),
	}
	return node, nil
}

func deltaPack(data []int64) {
	if len(data) < 2 {
		return
	}
	lastVal := data[0]
	for i := 1; i < len(data); i++ {
		data[i], lastVal = data[i]-lastVal, data[i]
	}
}

func deltaUnpack(data []int64) {
	if len(data) < 2 {
		return
	}
	for i := 1; i < len(data); i++ {
		data[i] = data[i] + data[i-1]
	}
}

func MarshalWay(way *osm.Way) ([]byte, error) {
	// TODO reuse Way to avoid make(Tags) for each way in tagsAsArray
	pbfWay := &Way{}
	deltaPack(way.Refs)
	pbfWay.Refs = way.Refs
	pbfWay.Tags = tagsAsArray(way.Tags)
	if way.Metadata != nil {
		pbfWay.Timestamp = way.Metadata.Timestamp.Unix();
	}
	return pbfWay.Marshal()
}

func UnmarshalWay(data []byte) (way *osm.Way, err error) {
	pbfWay := &Way{}
	err = pbfWay.Unmarshal(data)
	if err != nil {
		return nil, err
	}

	way = &osm.Way{}
	deltaUnpack(pbfWay.Refs)
	way.Refs = pbfWay.Refs
	way.Tags = tagsFromArray(pbfWay.Tags)
	way.Metadata = &osm.Metadata{
		Timestamp: time.Unix(pbfWay.Timestamp, 0),
	}
	return way, nil
}

func MarshalRelation(relation *osm.Relation) ([]byte, error) {
	pbfRelation := &Relation{}
	pbfRelation.MemberIds = make([]int64, len(relation.Members))
	pbfRelation.MemberTypes = make([]Relation_MemberType, len(relation.Members))
	pbfRelation.MemberRoles = make([]string, len(relation.Members))
	for i, m := range relation.Members {
		pbfRelation.MemberIds[i] = m.ID
		pbfRelation.MemberTypes[i] = Relation_MemberType(m.Type)
		pbfRelation.MemberRoles[i] = m.Role
	}
	pbfRelation.Tags = tagsAsArray(relation.Tags)
	if relation.Metadata != nil {
		pbfRelation.Timestamp = relation.Metadata.Timestamp.Unix();
	}
	return pbfRelation.Marshal()
}

func UnmarshalRelation(data []byte) (relation *osm.Relation, err error) {
	pbfRelation := &Relation{}
	err = pbfRelation.Unmarshal(data)
	if err != nil {
		return nil, err
	}

	relation = &osm.Relation{}
	relation.Members = make([]osm.Member, len(pbfRelation.MemberIds))
	for i := range pbfRelation.MemberIds {
		relation.Members[i].ID = pbfRelation.MemberIds[i]
		relation.Members[i].Type = osm.MemberType(pbfRelation.MemberTypes[i])
		relation.Members[i].Role = pbfRelation.MemberRoles[i]
	}
	//relation.Nodes = pbfRelation.Node
	relation.Tags = tagsFromArray(pbfRelation.Tags)
	relation.Metadata = &osm.Metadata{
		Timestamp: time.Unix(pbfRelation.Timestamp, 0),
	}
	return relation, nil
}
